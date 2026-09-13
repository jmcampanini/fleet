// Package query retrieves and orders issues and pull requests across repositories.
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmcampanini/fleet/internal/inventory"
	"github.com/jmcampanini/fleet/internal/process"
)

// Resource selects issues or pull requests.
type Resource string

// Resources Fleet can list.
const (
	ResourceIssues Resource = "issues"
	ResourcePRs    Resource = "prs"
)

// State filters items by lifecycle state.
type State string

// States accepted by --state; StateMerged applies only to pull requests.
const (
	StateAll    State = "all"
	StateClosed State = "closed"
	StateMerged State = "merged"
	StateOpen   State = "open"
)

// Sort names the event timestamp used for global ordering.
type Sort string

// Sorts accepted by --sort; SortMerged applies only to pull requests.
const (
	SortClosed  Sort = "closed"
	SortCreated Sort = "created"
	SortMerged  Sort = "merged"
	SortUpdated Sort = "updated"
)

// Order is the direction of the sort timestamp.
type Order string

// Orders accepted by --order.
const (
	OrderAsc  Order = "asc"
	OrderDesc Order = "desc"
)

// Options specifies a resource query and global result limit.
type Options struct {
	Resource Resource `json:"resource" help:"issues or prs."`
	State    State    `json:"state" help:"open, closed, all, or merged (prs only)."`
	Sort     Sort     `json:"sort" help:"created, updated, closed, or merged (prs only)."`
	Order    Order    `json:"order" help:"asc or desc."`
	Limit    int      `json:"limit" help:"Maximum items across the selection."`
}

// Validate fills the state-dependent default sort and rejects invalid
// controls. The limit is not a control; configuration validates it.
func (o *Options) Validate() error {
	if o.Resource != ResourceIssues && o.Resource != ResourcePRs {
		return fmt.Errorf("unknown resource %q", o.Resource)
	}
	if o.State != StateOpen && o.State != StateClosed && o.State != StateAll && (o.Resource != ResourcePRs || o.State != StateMerged) {
		return fmt.Errorf("invalid %s state %q", o.Resource, o.State)
	}
	if o.Sort == "" {
		o.Sort = map[State]Sort{StateOpen: SortCreated, StateClosed: SortClosed, StateMerged: SortMerged, StateAll: SortUpdated}[o.State]
	}
	if o.Sort != SortCreated && o.Sort != SortUpdated && o.Sort != SortClosed && o.Sort != SortMerged {
		return fmt.Errorf("invalid sort %q; use created, updated, closed, or merged (PRs only)", o.Sort)
	}
	if (o.Sort == SortClosed && o.State != StateClosed) || (o.Sort == SortMerged && (o.Resource != ResourcePRs || o.State != StateMerged)) {
		return fmt.Errorf("sort %q requires the matching state; merged applies only to prs", o.Sort)
	}
	if o.Order != OrderAsc && o.Order != OrderDesc {
		return fmt.Errorf("invalid order %q; use asc or desc", o.Order)
	}
	return nil
}

// Item retains the resource-specific state reason and event timestamps.
type Item struct {
	Repository  string     `json:"repository" help:"Configured complete identity."`
	Number      int        `json:"number" help:"Item number within the repository."`
	Title       string     `json:"title" help:"Item title."`
	URL         string     `json:"url" help:"Canonical URL; the deduplication key."`
	State       string     `json:"state" help:"open, closed, or merged."`
	StateReason string     `json:"state_reason,omitempty" help:"GitHub's issue state reason verbatim, such as COMPLETED or NOT_PLANNED. Omitted for pull requests."`
	CreatedAt   time.Time  `json:"created_at" help:"Creation timestamp."`
	UpdatedAt   time.Time  `json:"updated_at" help:"Last update timestamp."`
	ClosedAt    *time.Time `json:"closed_at" help:"Closure timestamp, or null while open."`
	MergedAt    *time.Time `json:"merged_at" help:"Merge timestamp, or null for issues and unmerged pull requests."`
}

// Timestamp returns the event chosen for ordering a validated query.
func (i Item) Timestamp(sort Sort) time.Time {
	switch sort {
	case SortUpdated:
		return i.UpdatedAt
	case SortClosed:
		if i.ClosedAt != nil {
			return *i.ClosedAt
		}
	case SortMerged:
		if i.MergedAt != nil {
			return *i.MergedAt
		}
	default:
		return i.CreatedAt
	}
	return time.Time{}
}

// RepositoryResult records query completeness separately from returned items.
type RepositoryResult struct {
	Repository string `json:"repository" help:"Configured complete identity."`
	Complete   bool   `json:"complete" help:"The repository established enough candidates for the requested ordering."`
	Error      string `json:"error,omitempty" help:"Failure reason, present only for incomplete repositories."`
}

// Report contains a globally limited result and per-repository failures.
type Report struct {
	Query        Options            `json:"query" help:"The validated controls in effect."`
	Complete     bool               `json:"complete" help:"Every repository established enough candidates for the requested global ordering. An incomplete report is not the newest or oldest across the selection."`
	Items        []Item             `json:"items" help:"Matching items after deduplication, ordering, and the limit."`
	Repositories []RepositoryResult `json:"repositories" help:"One entry per queried repository."`
}

// Client retrieves gh JSON with at most four repositories in flight.
type Client struct {
	Run process.Run
}

// List validates the options, queries each repository independently, and
// aggregates in inventory order. Invalid options produce an incomplete
// report that carries the validation error on every repository.
func (c Client) List(ctx context.Context, repos []inventory.Repository, options Options) Report {
	report := Report{Query: options, Complete: true, Items: []Item{}, Repositories: make([]RepositoryResult, len(repos))}
	if err := options.Validate(); err != nil {
		report.Complete = len(repos) == 0
		for index, repo := range repos {
			report.Repositories[index] = RepositoryResult{Repository: repo.ID, Error: err.Error()}
		}
		return report
	}
	// Validate filled the default sort; report the controls actually used.
	report.Query = options

	candidates := make([][]Item, len(repos))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(4, len(repos)) {
		workers.Go(func() {
			for index := range jobs {
				items, err := c.listRepository(ctx, repos[index].ID, options)
				candidates[index] = items
				result := RepositoryResult{Repository: repos[index].ID, Complete: err == nil}
				if err != nil {
					result.Error = err.Error()
				}
				report.Repositories[index] = result
			}
		})
	}
	for index := range repos {
		jobs <- index
	}
	close(jobs)
	workers.Wait()

	for index, result := range report.Repositories {
		report.Complete = report.Complete && result.Complete
		report.Items = append(report.Items, candidates[index]...)
	}
	report.Items = best(report.Items, options)
	return report
}

func (c Client) listRepository(ctx context.Context, id string, options Options) ([]Item, error) {
	resource := "issue"
	fields := "number,title,url,state,stateReason,createdAt,updatedAt,closedAt"
	if options.Resource == ResourcePRs {
		resource, fields = "pr", "number,title,url,state,createdAt,updatedAt,closedAt,mergedAt"
	}
	retrievalSort, retrievalOrder := options.Sort, options.Order
	if options.Sort == SortClosed || options.Sort == SortMerged {
		retrievalSort, retrievalOrder = SortUpdated, OrderDesc
	}
	var candidates []Item
	// gh list exposes a total limit, not a cursor. Expand the ordered prefix
	// by 100 until the bound proves completeness or the search cap is reached.
	for limit := 100; limit <= 1000; limit += 100 {
		args := []string{resource, "list", "--repo", id, "--state", string(options.State), "--limit", strconv.Itoa(limit), "--json", fields}
		if retrievalSort != SortCreated || retrievalOrder != OrderDesc {
			args = append(args, "--search", "sort:"+string(retrievalSort)+"-"+string(retrievalOrder))
		}
		body, err := c.Run(ctx, "", "gh", args...)
		if err != nil {
			host, _, _ := strings.Cut(id, "/")
			return candidates, fmt.Errorf("query %q on host %q with gh: %w", id, host, err)
		}
		page, err := decode(body, id, options)
		if err != nil {
			return candidates, err
		}
		for index := 1; index < len(page); index++ {
			comparison := page[index-1].Timestamp(retrievalSort).Compare(page[index].Timestamp(retrievalSort))
			if (retrievalOrder == OrderDesc && comparison < 0) || (retrievalOrder == OrderAsc && comparison > 0) {
				return candidates, fmt.Errorf("query %q returned items outside the requested retrieval order; cannot prove completeness", id)
			}
		}
		var last Item
		if len(page) > 0 {
			last = page[len(page)-1]
		}
		candidates = best(page, options)
		if len(page) < limit {
			return candidates, nil
		}
		if len(candidates) < options.Limit {
			continue
		}
		threshold := candidates[len(candidates)-1].Timestamp(options.Sort)
		if options.Sort == SortClosed || options.Sort == SortMerged {
			// Strict inequality also resolves item-number ties at the boundary.
			if options.Order == OrderDesc && threshold.After(last.UpdatedAt) {
				return candidates, nil
			}
		} else if (options.Order == OrderDesc && threshold.After(last.Timestamp(options.Sort))) || (options.Order == OrderAsc && threshold.Before(last.Timestamp(options.Sort))) {
			return candidates, nil
		}
	}
	return candidates, fmt.Errorf("incomplete: reached the 1000-candidate cap before proving the requested ordering; reduce the limit or query directly with gh")
}

func decode(body, id string, options Options) ([]Item, error) {
	var raw []struct {
		Number      int
		Title       string
		URL         string
		State       string
		StateReason string
		CreatedAt   time.Time
		UpdatedAt   time.Time
		ClosedAt    *time.Time
		MergedAt    *time.Time
	}
	if !strings.HasPrefix(strings.TrimSpace(body), "[") {
		return nil, fmt.Errorf("query %q returned a non-array JSON payload", id)
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, fmt.Errorf("decode query %q: %w", id, err)
	}
	var items []Item
	for _, row := range raw {
		item := Item{Repository: id, Number: row.Number, Title: row.Title, URL: row.URL, State: strings.ToLower(row.State), StateReason: row.StateReason, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, ClosedAt: row.ClosedAt, MergedAt: row.MergedAt}
		if item.State != string(StateOpen) && item.State != string(StateClosed) && (options.Resource != ResourcePRs || item.State != string(StateMerged)) {
			return nil, fmt.Errorf("query %q returned an unexpected state %q", id, item.State)
		}
		if item.Number <= 0 || item.URL == "" || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() || item.Timestamp(options.Sort).IsZero() {
			return nil, fmt.Errorf("query %q returned an item missing identity or required timestamps", id)
		}
		if (item.ClosedAt != nil && item.ClosedAt.After(item.UpdatedAt)) || (item.MergedAt != nil && item.MergedAt.After(item.UpdatedAt)) {
			return nil, fmt.Errorf("query %q returned event timestamps after updatedAt; cannot prove ordering", id)
		}
		items = append(items, item)
	}
	return items, nil
}

func best(items []Item, options Options) []Item {
	slices.SortFunc(items, func(a, b Item) int {
		order := a.Timestamp(options.Sort).Compare(b.Timestamp(options.Sort))
		if order != 0 {
			if options.Order == OrderDesc {
				return -order
			}
			return order
		}
		if order := strings.Compare(a.Repository, b.Repository); order != 0 {
			return order
		}
		return a.Number - b.Number
	})
	seen := make(map[string]bool)
	selected := make([]Item, 0, min(options.Limit, len(items)))
	for _, item := range items {
		if !seen[item.URL] {
			seen[item.URL] = true
			selected = append(selected, item)
		}
		if len(selected) == options.Limit {
			break
		}
	}
	return selected
}
