package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmcampanini/fleet/internal/inventory"
)

func row(number int, created, updated, closed, merged string) map[string]any {
	r := map[string]any{"number": number, "title": "an item", "url": fmt.Sprintf("https://github.com/a/b/issues/%d", number), "state": "CLOSED", "stateReason": "COMPLETED", "createdAt": created, "updatedAt": updated}
	if closed != "" {
		r["closedAt"] = closed
	}
	if merged != "" {
		r["mergedAt"] = merged
		r["state"] = "MERGED"
	}
	return r
}

func payload(t *testing.T, rows []map[string]any) string {
	t.Helper()
	data, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEventOrderingPagesPastLaterComments(t *testing.T) {
	for _, field := range []Sort{SortClosed, SortMerged} {
		t.Run(string(field), func(t *testing.T) {
			var rows []map[string]any
			for number := 1; number <= 100; number++ {
				rows = append(rows, row(number, "2025-01-01T00:00:00Z", "2026-09-10T00:00:00Z", "2025-02-01T00:00:00Z", ""))
				if field == SortMerged {
					rows[number-1]["mergedAt"] = "2025-02-01T00:00:00Z"
				}
			}
			newest := row(101, "2025-01-01T00:00:00Z", "2026-09-09T00:00:00Z", "2026-09-09T00:00:00Z", "")
			if field == SortMerged {
				newest["mergedAt"] = "2026-09-09T00:00:00Z"
			}
			rows = append(rows, newest)
			calls := 0
			client := Client{Run: func(_ context.Context, _, program string, args ...string) (string, error) {
				calls++
				if program != "gh" || args[1] != "list" || !strings.Contains(strings.Join(args, " "), "sort:updated-desc") {
					t.Fatalf("query arguments = %v", args)
				}
				limit := 0
				for i, arg := range args {
					if arg == "--limit" {
						limit, _ = strconv.Atoi(args[i+1])
					}
				}
				return payload(t, rows[:min(limit, len(rows))]), nil
			}}
			options := Options{Resource: ResourceIssues, State: StateClosed, Sort: field, Order: OrderDesc, Limit: 1}
			if field == SortMerged {
				options.Resource, options.State = ResourcePRs, StateMerged
			}
			report := client.List(t.Context(), []inventory.Repository{{ID: "github.com/a/b"}}, options, Cursor{})
			if !report.Complete || calls != 2 || len(report.Items) != 1 || report.Items[0].Number != 101 {
				t.Fatalf("List() = %+v after %d calls", report, calls)
			}
		})
	}
}

func TestBoundUsesRetrievalOrderAndContinuesThroughTies(t *testing.T) {
	var rows []map[string]any
	for number := 100; number > 0; number-- {
		rows = append(rows, row(number+1, "2025-01-01T00:00:00Z", "2026-09-09T00:00:00Z", "2026-09-09T00:00:00Z", ""))
	}
	rows = append(rows, row(1, "2025-01-01T00:00:00Z", "2026-09-09T00:00:00Z", "2026-09-09T00:00:00Z", ""))
	calls := 0
	client := Client{Run: func(_ context.Context, _, _ string, _ ...string) (string, error) {
		calls++
		if calls == 1 {
			return payload(t, rows[:100]), nil
		}
		return payload(t, rows), nil
	}}
	report := client.List(t.Context(), []inventory.Repository{{ID: "github.com/a/b"}}, Options{Resource: ResourceIssues, State: StateClosed, Sort: SortClosed, Order: OrderDesc, Limit: 1}, Cursor{})
	if !report.Complete || calls != 2 || report.Items[0].Number != 1 {
		t.Fatalf("tie report = %+v after %d calls", report, calls)
	}
}

func TestCapAndQueryFailureRemainPartial(t *testing.T) {
	var calls atomic.Int32
	client := Client{Run: func(_ context.Context, _, _ string, args ...string) (string, error) {
		calls.Add(1)
		if strings.Contains(strings.Join(args, " "), "github.com/fail/repo") {
			return "", errors.New("authentication failed")
		}
		limit := 0
		for i, arg := range args {
			if arg == "--limit" {
				limit, _ = strconv.Atoi(args[i+1])
			}
		}
		var rows []map[string]any
		for number := 1; number <= limit; number++ {
			rows = append(rows, row(number, "2025-01-01T00:00:00Z", "2026-09-10T00:00:00Z", "2025-02-01T00:00:00Z", ""))
		}
		return payload(t, rows), nil
	}}
	report := client.List(t.Context(), []inventory.Repository{{ID: "github.com/a/b"}, {ID: "github.com/fail/repo"}}, Options{Resource: ResourceIssues, State: StateClosed, Sort: SortClosed, Order: OrderDesc, Limit: 2}, Cursor{})
	if report.Complete || len(report.Items) != 2 || report.NextCursor == "" || calls.Load() != 11 || !errorContains(report.Repositories[0].Err, "cap") || !errorContains(report.Repositories[1].Err, "authentication failed") {
		t.Fatalf("List() = %+v, calls=%d", report, calls.Load())
	}
}

func TestGlobalLimitURLDeduplicationAndTieOrder(t *testing.T) {
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	items := []Item{
		{Repository: "github.com/z/b", Number: 1, URL: "z1", CreatedAt: stamp},
		{Repository: "github.com/a/b", Number: 2, URL: "a2", CreatedAt: stamp},
		{Repository: "github.com/a/b", Number: 1, URL: "a1", CreatedAt: stamp},
		{Repository: "github.com/a/b", Number: 1, URL: "a1", CreatedAt: stamp},
	}
	for _, order := range []Order{OrderAsc, OrderDesc} {
		got := best(items, Options{Sort: SortCreated, Order: order, Limit: 2})
		if len(got) != 2 || got[0].URL != "a1" || got[1].URL != "a2" {
			t.Errorf("best(%q) = %+v", order, got)
		}
	}
}

func TestControls(t *testing.T) {
	for state, sort := range map[State]Sort{StateOpen: SortCreated, StateClosed: SortClosed, StateAll: SortUpdated, StateMerged: SortMerged} {
		o := Options{Resource: ResourcePRs, State: state, Order: OrderDesc, Limit: 15}
		if err := o.Validate(); err != nil || o.Sort != sort {
			t.Errorf("Validate(%q) = %+v, %v", state, o, err)
		}
	}
	for _, o := range []Options{
		{Resource: ResourceIssues, State: StateMerged, Sort: SortMerged, Order: OrderDesc, Limit: 15},
		{Resource: ResourceIssues, State: StateOpen, Sort: SortClosed, Order: OrderDesc, Limit: 15},
		{Resource: ResourcePRs, State: StateClosed, Sort: SortMerged, Order: OrderDesc, Limit: 15},
		{Resource: ResourceIssues, State: StateOpen, Sort: SortCreated, Order: "invalid", Limit: 15},
	} {
		if err := o.Validate(); err == nil {
			t.Errorf("Validate(%+v) succeeded", o)
		}
	}
}

func TestClosedPRsRetainBothOutcomes(t *testing.T) {
	rows := []map[string]any{
		row(1, "2025-01-01T00:00:00Z", "2026-09-09T00:00:00Z", "2026-09-09T00:00:00Z", "2026-09-09T00:00:00Z"),
		row(2, "2025-01-01T00:00:00Z", "2026-09-10T00:00:00Z", "2026-09-08T00:00:00Z", ""),
	}
	items, err := decode(payload(t, rows), "github.com/a/b", Options{Resource: ResourcePRs, State: StateClosed, Sort: SortClosed})
	if err != nil || len(items) != 2 || items[0].State != "merged" || items[1].State != "closed" || items[1].MergedAt != nil {
		t.Fatalf("decode = %+v, %v", items, err)
	}
}

func TestListRejectsInvalidOptionsBeforeQuerying(t *testing.T) {
	client := Client{Run: func(_ context.Context, _, _ string, _ ...string) (string, error) {
		t.Fatal("List ran gh with invalid options")
		return "", nil
	}}

	report := client.List(t.Context(), []inventory.Repository{{ID: "github.com/a/b"}}, Options{Resource: ResourceIssues, State: StateOpen, Sort: SortMerged, Order: OrderDesc, Limit: 15}, Cursor{})

	if report.Complete || len(report.Items) != 0 || len(report.Repositories) != 1 || !errorContains(report.Repositories[0].Err, "requires the matching state") {
		t.Fatalf("List() with invalid options = %+v, want an incomplete report carrying the validation error", report)
	}
}

func errorContains(err error, text string) bool {
	return err != nil && strings.Contains(err.Error(), text)
}

// listing serves gh list for created-descending queries from fixed items
// per repository, honoring --limit, and fails the repositories in failing.
type listing struct {
	failing map[string]bool
	limits  []int
	mu      sync.Mutex
	rows    map[string][]map[string]any
}

func (l *listing) run(t *testing.T) func(context.Context, string, string, ...string) (string, error) {
	return func(_ context.Context, _, _ string, args ...string) (string, error) {
		var repo string
		limit := 0
		for i, arg := range args {
			switch arg {
			case "--repo":
				repo = args[i+1]
			case "--limit":
				limit, _ = strconv.Atoi(args[i+1])
			case "--search":
				t.Errorf("created descending query used --search: %v", args)
			}
		}
		l.mu.Lock()
		l.limits = append(l.limits, limit)
		l.mu.Unlock()
		if l.failing[repo] {
			return "", errors.New("authentication failed")
		}
		rows := l.rows[repo]
		return payload(t, rows[:min(limit, len(rows))]), nil
	}
}

// createdRows returns count open items of repo, newest first, one hour
// apart and ending at the given hour of 2026-09-01.
func createdRows(repo string, count, newest int) []map[string]any {
	var rows []map[string]any
	for index := range count {
		stamp := time.Date(2026, 9, 1, newest-index, 0, 0, 0, time.UTC).Format(time.RFC3339)
		rows = append(rows, map[string]any{"number": index + 1, "title": "an item", "url": fmt.Sprintf("https://%s/issues/%d", repo, index+1), "state": "OPEN", "createdAt": stamp, "updatedAt": stamp})
	}
	return rows
}

func urls(items []Item) []string {
	var out []string
	for _, item := range items {
		out = append(out, item.URL)
	}
	return out
}

func TestLimitCutReportsHasMoreAndCursorResumes(t *testing.T) {
	repos := []inventory.Repository{{ID: "github.com/a/one"}, {ID: "github.com/a/two"}}
	source := &listing{rows: map[string][]map[string]any{
		"github.com/a/one": createdRows("github.com/a/one", 3, 20),
		"github.com/a/two": createdRows("github.com/a/two", 2, 19),
	}}
	client := Client{Run: source.run(t)}
	options := Options{Resource: ResourceIssues, State: StateOpen, Order: OrderDesc, Limit: 2}
	if err := options.Validate(); err != nil {
		t.Fatal(err)
	}

	var seen []string
	cursor := Cursor{}
	for page := 1; ; page++ {
		report := client.List(t.Context(), repos, options, cursor)
		if !report.Complete {
			t.Fatalf("page %d = %+v, want complete", page, report)
		}
		seen = append(seen, urls(report.Items)...)
		if !report.HasMore {
			if report.NextCursor != "" {
				t.Errorf("final page nextCursor = %q, want none", report.NextCursor)
			}
			break
		}
		if len(report.Items) != options.Limit || report.NextCursor == "" {
			t.Fatalf("page %d = %+v, want %d items and a cursor", page, report, options.Limit)
		}
		var err error
		if cursor, err = ParseCursor(report.NextCursor, options, repos); err != nil {
			t.Fatalf("ParseCursor(page %d) = %v", page, err)
		}
	}

	want := []string{"https://github.com/a/one/issues/1", "https://github.com/a/one/issues/2", "https://github.com/a/two/issues/1", "https://github.com/a/one/issues/3", "https://github.com/a/two/issues/2"}
	if strings.Join(seen, " ") != strings.Join(want, " ") {
		t.Errorf("pages returned %v, want %v", seen, want)
	}
}

func TestExactLimitReportsNoMore(t *testing.T) {
	source := &listing{rows: map[string][]map[string]any{"github.com/a/one": createdRows("github.com/a/one", 2, 20)}}
	report := Client{Run: source.run(t)}.List(t.Context(), []inventory.Repository{{ID: "github.com/a/one"}}, Options{Resource: ResourceIssues, State: StateOpen, Order: OrderDesc, Limit: 2}, Cursor{})

	if !report.Complete || report.HasMore || report.NextCursor != "" || len(report.Items) != 2 {
		t.Errorf("List() = %+v, want two items and no more", report)
	}
}

func TestCursorResumesFailedRepositoryWithoutRepeats(t *testing.T) {
	repos := []inventory.Repository{{ID: "github.com/a/one"}, {ID: "github.com/a/two"}}
	source := &listing{
		failing: map[string]bool{"github.com/a/two": true},
		rows: map[string][]map[string]any{
			"github.com/a/one": createdRows("github.com/a/one", 3, 20),
			"github.com/a/two": createdRows("github.com/a/two", 2, 21),
		},
	}
	client := Client{Run: source.run(t)}
	options := Options{Resource: ResourceIssues, State: StateOpen, Order: OrderDesc, Limit: 2}
	if err := options.Validate(); err != nil {
		t.Fatal(err)
	}

	first := client.List(t.Context(), repos, options, Cursor{})
	if first.Complete || !first.HasMore || first.NextCursor == "" || first.Repositories[1].Complete {
		t.Fatalf("failed page = %+v, want incomplete with a cursor", first)
	}
	cursor, err := ParseCursor(first.NextCursor, options, repos)
	if err != nil {
		t.Fatal(err)
	}
	source.failing = nil
	second := client.List(t.Context(), repos, options.withLimit(10), cursor)

	if !second.Complete || second.HasMore {
		t.Fatalf("resumed page = %+v, want complete", second)
	}
	got := append(urls(first.Items), urls(second.Items)...)
	want := []string{"https://github.com/a/one/issues/1", "https://github.com/a/one/issues/2", "https://github.com/a/two/issues/1", "https://github.com/a/two/issues/2", "https://github.com/a/one/issues/3"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("pages returned %v, want %v", got, want)
	}
}

func (o Options) withLimit(limit int) Options {
	o.Limit = limit
	return o
}

func TestAllPagesPastTheCapOnlyForGraphQLListing(t *testing.T) {
	repo := inventory.Repository{ID: "github.com/a/one"}
	source := &listing{rows: map[string][]map[string]any{repo.ID: createdRows(repo.ID, 1500, 2000)}}
	report := Client{Run: source.run(t)}.List(t.Context(), []inventory.Repository{repo}, Options{Resource: ResourceIssues, State: StateOpen, Order: OrderDesc, All: true}, Cursor{})

	if !report.Complete || report.HasMore || report.NextCursor != "" || len(report.Items) != 1500 {
		t.Errorf("List(--all) = complete %t, hasMore %t, %d items, want all 1500", report.Complete, report.HasMore, len(report.Items))
	}
	if fmt.Sprint(source.limits) != "[100 200 400 800 1600]" {
		t.Errorf("gh limits = %v, want doubling until exhausted", source.limits)
	}

	var calls int
	search := Client{Run: func(_ context.Context, _, _ string, args ...string) (string, error) {
		calls++
		limit := 0
		for i, arg := range args {
			if arg == "--limit" {
				limit, _ = strconv.Atoi(args[i+1])
			}
		}
		rows := createdRows(repo.ID, 1500, 2000)
		return payload(t, rows[:limit]), nil
	}}
	capped := search.List(t.Context(), []inventory.Repository{repo}, Options{Resource: ResourceIssues, State: StateOpen, Sort: SortUpdated, Order: OrderDesc, All: true}, Cursor{})
	if capped.Complete || len(capped.Items) != candidateCap || capped.NextCursor == "" || !errorContains(capped.Repositories[0].Err, "cap") || calls != 5 {
		t.Errorf("List(--all, search) = %+v after %d calls, want a failed read at the cap", capped, calls)
	}
}

func TestParseCursorRejectsForeignAndMalformedValues(t *testing.T) {
	repos := []inventory.Repository{{ID: "github.com/a/one"}}
	options := Options{Resource: ResourceIssues, State: StateOpen, Sort: SortCreated, Order: OrderDesc, Limit: 1}
	key := Key{Time: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Repository: "github.com/a/one", Number: 3}
	text := Cursor{After: &key}.encode(options, repos)

	if cursor, err := ParseCursor(text, options.withLimit(50), repos); err != nil || cursor.After == nil || cursor.After.Number != 3 {
		t.Errorf("ParseCursor(other limit) = %+v, %v, want the encoded position", cursor, err)
	}
	closed := options
	closed.State, closed.Sort = StateClosed, SortClosed
	for name, err := range map[string]error{
		"other state":     second(ParseCursor(text, closed, repos)),
		"other selection": second(ParseCursor(text, options, append(repos, inventory.Repository{ID: "github.com/a/two"}))),
	} {
		if !errors.Is(err, ErrCursorQuery) {
			t.Errorf("ParseCursor(%s) error = %v, want ErrCursorQuery", name, err)
		}
	}
	for _, bad := range []string{"", "not base64!", "e30", text + "x"} {
		if _, err := ParseCursor(bad, options, repos); !errors.Is(err, ErrCursorInvalid) {
			t.Errorf("ParseCursor(%q) error = %v, want ErrCursorInvalid", bad, err)
		}
	}
}

func second[T any](_ T, err error) error {
	return err
}

func TestCursorSkipsURLsKeptUnderAnotherIdentity(t *testing.T) {
	// Two configured identities that resolve to one GitHub repository return
	// the same URLs; deduplication keeps the copy under the first identity.
	repos := []inventory.Repository{{ID: "github.com/A/repo"}, {ID: "github.com/a/repo"}}
	rows := createdRows("github.com/a/repo", 3, 20)
	source := &listing{rows: map[string][]map[string]any{repos[0].ID: rows, repos[1].ID: rows}}
	client := Client{Run: source.run(t)}
	options := Options{Resource: ResourceIssues, State: StateOpen, Order: OrderDesc, Limit: 1}
	if err := options.Validate(); err != nil {
		t.Fatal(err)
	}

	var seen []string
	cursor := Cursor{}
	for page := 1; page <= 5; page++ {
		report := client.List(t.Context(), repos, options, cursor)
		seen = append(seen, urls(report.Items)...)
		if !report.HasMore {
			break
		}
		var err error
		if cursor, err = ParseCursor(report.NextCursor, options, repos); err != nil {
			t.Fatal(err)
		}
	}

	want := []string{"https://github.com/a/repo/issues/1", "https://github.com/a/repo/issues/2", "https://github.com/a/repo/issues/3"}
	if strings.Join(seen, " ") != strings.Join(want, " ") {
		t.Errorf("pages returned %v, want %v", seen, want)
	}
}

func TestCursorPassesTheCapOnGraphQLListing(t *testing.T) {
	repo := inventory.Repository{ID: "github.com/a/one"}
	rows := createdRows(repo.ID, 1200, 2000)
	source := &listing{rows: map[string][]map[string]any{repo.ID: rows}}
	stamp, err := time.Parse(time.RFC3339, rows[994]["createdAt"].(string))
	if err != nil {
		t.Fatal(err)
	}
	cursor := Cursor{After: &Key{Time: stamp, Repository: repo.ID, Number: 995}}

	report := Client{Run: source.run(t)}.List(t.Context(), []inventory.Repository{repo}, Options{Resource: ResourceIssues, State: StateOpen, Sort: SortCreated, Order: OrderDesc, Limit: 10}, cursor)

	if !report.Complete || !report.HasMore || len(report.Items) != 10 || report.Items[0].Number != 996 {
		t.Errorf("List(--cursor past 1000) = complete %t, hasMore %t, %d items, want items 996-1005", report.Complete, report.HasMore, len(report.Items))
	}
}

func TestMaximumLimitReturnsEveryItem(t *testing.T) {
	repo := inventory.Repository{ID: "github.com/a/one"}
	source := &listing{rows: map[string][]map[string]any{repo.ID: createdRows(repo.ID, 3, 20)}}

	report := Client{Run: source.run(t)}.List(t.Context(), []inventory.Repository{repo}, Options{Resource: ResourceIssues, State: StateOpen, Order: OrderDesc, Limit: math.MaxInt}, Cursor{})

	if !report.Complete || report.HasMore || len(report.Items) != 3 {
		t.Errorf("List(math.MaxInt) = %+v, want three items and no more", report)
	}
}
