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

// Options specifies a validated resource query and global result limit.
type Options struct {
	Resource string `json:"resource"`
	State    string `json:"state"`
	Sort     string `json:"sort"`
	Order    string `json:"order"`
	Limit    int    `json:"limit"`
}

// Validate fills the state-dependent default sort and rejects invalid controls.
func (o *Options) Validate() error {
	if o.Resource != "issues" && o.Resource != "prs" {
		return fmt.Errorf("unknown resource %q", o.Resource)
	}
	if o.State != "open" && o.State != "closed" && o.State != "all" && (o.Resource != "prs" || o.State != "merged") {
		return fmt.Errorf("invalid %s state %q", o.Resource, o.State)
	}
	if o.Sort == "" {
		o.Sort = map[string]string{"open": "created", "closed": "closed", "merged": "merged", "all": "updated"}[o.State]
	}
	if o.Sort != "created" && o.Sort != "updated" && o.Sort != "closed" && o.Sort != "merged" {
		return fmt.Errorf("invalid sort %q; use created, updated, closed, or merged (PRs only)", o.Sort)
	}
	if (o.Sort == "closed" && o.State != "closed") || (o.Sort == "merged" && (o.Resource != "prs" || o.State != "merged")) {
		return fmt.Errorf("sort %q requires the matching state; merged applies only to prs", o.Sort)
	}
	if o.Order != "asc" && o.Order != "desc" {
		return fmt.Errorf("invalid order %q; use asc or desc", o.Order)
	}
	if o.Limit <= 0 {
		return fmt.Errorf("query limit must be positive")
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
func (i Item) Timestamp(sort string) time.Time {
	switch sort {
	case "updated":
		return i.UpdatedAt
	case "closed":
		if i.ClosedAt != nil {
			return *i.ClosedAt
		}
	case "merged":
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
	Repository string `json:"repository"`
	Complete   bool   `json:"complete"`
	Error      string `json:"error,omitempty"`
}

// Report contains a globally limited result and per-repository failures.
type Report struct {
	Query        Options            `json:"query"`
	Complete     bool               `json:"complete"`
	Items        []Item             `json:"items"`
	Repositories []RepositoryResult `json:"repositories"`
}

// Client retrieves gh JSON with at most four repositories in flight.
type Client struct {
	Run process.Run
}

// List queries each repository independently and aggregates in inventory order.
func (c Client) List(ctx context.Context, repos []inventory.Repository, options Options) Report {
	report := Report{Query: options, Complete: true, Items: []Item{}, Repositories: make([]RepositoryResult, len(repos))}
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
	if options.Resource == "prs" {
		resource, fields = "pr", "number,title,url,state,createdAt,updatedAt,closedAt,mergedAt"
	}
	retrievalSort, retrievalOrder := options.Sort, options.Order
	if options.Sort == "closed" || options.Sort == "merged" {
		retrievalSort, retrievalOrder = "updated", "desc"
	}
	var candidates []Item
	// gh list exposes a total limit, not a cursor. Expand the ordered prefix
	// by 100 until the bound proves completeness or the search cap is reached.
	for limit := 100; limit <= 1000; limit += 100 {
		args := []string{resource, "list", "--repo", id, "--state", options.State, "--limit", strconv.Itoa(limit), "--json", fields}
		if retrievalSort != "created" || retrievalOrder != "desc" {
			args = append(args, "--search", "sort:"+retrievalSort+"-"+retrievalOrder)
		}
		body, err := c.Run(ctx, "", "gh", args...)
		if err != nil {
			host, _, _ := strings.Cut(id, "/")
			return candidates, fmt.Errorf("query %q on host %q (authenticated GitHub or GitHub Enterprise required): %w", id, host, err)
		}
		page, err := decode(body, id, options)
		if err != nil {
			return candidates, err
		}
		for index := 1; index < len(page); index++ {
			comparison := page[index-1].Timestamp(retrievalSort).Compare(page[index].Timestamp(retrievalSort))
			if (retrievalOrder == "desc" && comparison < 0) || (retrievalOrder == "asc" && comparison > 0) {
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
		if options.Sort == "closed" || options.Sort == "merged" {
			// Strict inequality also resolves item-number ties at the boundary.
			if options.Order == "desc" && threshold.After(last.UpdatedAt) {
				return candidates, nil
			}
		} else if (options.Order == "desc" && threshold.After(last.Timestamp(options.Sort))) || (options.Order == "asc" && threshold.Before(last.Timestamp(options.Sort))) {
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
		if item.State != "open" && item.State != "closed" && (options.Resource != "prs" || item.State != "merged") {
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
			if options.Order == "desc" {
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
