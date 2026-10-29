// Package query retrieves and orders issues and pull requests across repositories.
package query

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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

// Options specifies a resource query and its global result limit. Limit is
// the maximum number of items to return; All lifts it, and Limit is then 0.
type Options struct {
	Resource Resource `json:"resource"`
	State    State    `json:"state"`
	Sort     Sort     `json:"sort"`
	Order    Order    `json:"order"`
	Limit    int      `json:"limit"`
	All      bool     `json:"all"`
}

// candidateCap bounds the candidates gh search returns for one repository.
const candidateCap = 1000

// Validate fills the state-dependent default sort and rejects invalid
// controls. Limit and All are not controls; the command validates them.
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
	Repository  string     `json:"repository"`
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	State       string     `json:"state"`
	StateReason string     `json:"state_reason,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	MergedAt    *time.Time `json:"merged_at"`
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

// RepositoryResult records whether one repository was read completely. Err
// explains a failed read; the JSON report carries no error text, so the
// command prints Err as a diagnostic instead.
type RepositoryResult struct {
	Repository string `json:"repository"`
	Complete   bool   `json:"complete"`
	Err        error  `json:"-"`
}

// Report contains a globally limited result and per-repository completeness.
// HasMore reports that the limit cut the result while more items match.
// NextCursor is set when HasMore is true or Complete is false.
type Report struct {
	Query        Options            `json:"query"`
	Complete     bool               `json:"complete"`
	HasMore      bool               `json:"hasMore"`
	NextCursor   string             `json:"nextCursor,omitempty"`
	Items        []Item             `json:"items"`
	Repositories []RepositoryResult `json:"repositories"`
}

// Client retrieves gh JSON with at most four repositories in flight.
type Client struct {
	Run process.Run
}

// List validates the options, queries each repository independently from
// its position in cursor, and aggregates in inventory order. A failed
// repository keeps the items gathered before its failure, and nextCursor
// resumes it from where this page started it. Invalid options produce an
// incomplete report that carries the validation error on every repository.
func (c Client) List(ctx context.Context, repos []inventory.Repository, options Options, cursor Cursor) Report {
	report := Report{Query: options, Complete: true, Items: []Item{}, Repositories: make([]RepositoryResult, len(repos))}
	if err := options.Validate(); err != nil {
		report.Complete = len(repos) == 0
		for index, repo := range repos {
			report.Repositories[index] = RepositoryResult{Repository: repo.ID, Err: err}
		}
		return report
	}
	// Validate filled the default sort; report the controls actually used.
	report.Query = options

	// Retrieving one item past the limit proves whether more items exist.
	// A limit of math.MaxInt already covers every item.
	retrieval := options
	if !options.All && options.Limit < math.MaxInt {
		retrieval.Limit++
	}
	candidates := make([][]Item, len(repos))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(4, len(repos)) {
		workers.Go(func() {
			for index := range jobs {
				items, err := c.listRepository(ctx, repos[index].ID, retrieval, cursor.position(repos[index].ID))
				candidates[index] = items
				report.Repositories[index] = RepositoryResult{Repository: repos[index].ID, Complete: err == nil, Err: err}
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
	report.Items = best(report.Items, retrieval)
	if !options.All && len(report.Items) > options.Limit {
		report.HasMore = true
		report.Items = report.Items[:options.Limit]
	}
	if report.HasMore || !report.Complete {
		report.NextCursor = cursor.next(repos, report, candidates).encode(options, repos)
	}
	return report
}

// listRepository returns the repository's best items after position. On an
// error it also returns the candidates gathered before the failure.
func (c Client) listRepository(ctx context.Context, id string, options Options, position *Key) ([]Item, error) {
	resource := "issue"
	fields := "number,title,url,state,stateReason,createdAt,updatedAt,closedAt"
	if options.Resource == ResourcePRs {
		resource, fields = "pr", "number,title,url,state,createdAt,updatedAt,closedAt,mergedAt"
	}
	retrievalSort, retrievalOrder := options.Sort, options.Order
	if options.Sort == SortClosed || options.Sort == SortMerged {
		retrievalSort, retrievalOrder = SortUpdated, OrderDesc
	}
	// Ordinary GraphQL listing serves only created descending. Every other
	// retrieval order starts with gh search, which stops at candidateCap.
	// Fall back to GraphQL listing when search cannot prove completeness.
	search := retrievalSort != SortCreated || retrievalOrder != OrderDesc
	var candidates []Item
	// gh list exposes a total limit, not a cursor. Expand the ordered prefix
	// by 100 until the bound proves completeness, or double it when the
	// prefix may run long. Search stops at its cap; GraphQL listing does not.
	for limit := 100; ; {
		args := []string{resource, "list", "--repo", id, "--state", string(options.State), "--limit", strconv.Itoa(limit), "--json", fields}
		if search {
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
		var remaining []Item
		for _, item := range page {
			if after(item, position, options) {
				remaining = append(remaining, item)
			}
		}
		candidates = best(remaining, options)
		if len(page) < limit {
			return candidates, nil
		}
		canBound := search || (options.Sort == SortCreated && options.Order == OrderDesc)
		if !options.All && canBound && len(candidates) >= options.Limit && bounded(candidates[len(candidates)-1], last, options) {
			return candidates, nil
		}
		if search && limit >= candidateCap {
			search = false
			retrievalSort, retrievalOrder = SortCreated, OrderDesc
			continue
		}
		if !search || options.All || position != nil {
			limit *= 2
		} else {
			limit += 100
		}
		if search {
			limit = min(limit, candidateCap)
		}
	}
}

// bounded reports whether the unseen items, which follow last in retrieval
// order, all sort after threshold, so the candidates are proven.
func bounded(threshold, last Item, options Options) bool {
	at := threshold.Timestamp(options.Sort)
	if options.Sort == SortClosed || options.Sort == SortMerged {
		// Strict inequality also resolves item-number ties at the boundary.
		return options.Order == OrderDesc && at.After(last.UpdatedAt)
	}
	return (options.Order == OrderDesc && at.After(last.Timestamp(options.Sort))) || (options.Order == OrderAsc && at.Before(last.Timestamp(options.Sort)))
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

// best orders items by their key, drops repeated URLs, and keeps at most
// options.Limit unless options.All is set.
func best(items []Item, options Options) []Item {
	slices.SortFunc(items, func(a, b Item) int {
		return compareKeys(a.key(options.Sort), b.key(options.Sort), options.Order)
	})
	capacity := len(items)
	if !options.All {
		capacity = min(options.Limit, capacity)
	}
	seen := make(map[string]bool)
	selected := make([]Item, 0, capacity)
	for _, item := range items {
		if !seen[item.URL] {
			seen[item.URL] = true
			selected = append(selected, item)
		}
		if !options.All && len(selected) == options.Limit {
			break
		}
	}
	return selected
}
