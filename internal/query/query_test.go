package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	for _, field := range []string{"closed", "merged"} {
		t.Run(field, func(t *testing.T) {
			var rows []map[string]any
			for number := 1; number <= 100; number++ {
				rows = append(rows, row(number, "2025-01-01T00:00:00Z", "2026-09-10T00:00:00Z", "2025-02-01T00:00:00Z", ""))
				if field == "merged" {
					rows[number-1]["mergedAt"] = "2025-02-01T00:00:00Z"
				}
			}
			newest := row(101, "2025-01-01T00:00:00Z", "2026-09-09T00:00:00Z", "2026-09-09T00:00:00Z", "")
			if field == "merged" {
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
			options := Options{Resource: "issues", State: "closed", Sort: field, Order: "desc", Limit: 1}
			if field == "merged" {
				options.Resource, options.State = "prs", "merged"
			}
			report := client.List(t.Context(), []inventory.Repository{{ID: "github.com/a/b"}}, options)
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
	report := client.List(t.Context(), []inventory.Repository{{ID: "github.com/a/b"}}, Options{Resource: "issues", State: "closed", Sort: "closed", Order: "desc", Limit: 1})
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
	report := client.List(t.Context(), []inventory.Repository{{ID: "github.com/a/b"}, {ID: "github.com/fail/repo"}}, Options{Resource: "issues", State: "closed", Sort: "closed", Order: "desc", Limit: 2})
	if report.Complete || len(report.Items) != 2 || calls.Load() != 11 || !strings.Contains(report.Repositories[0].Error, "cap") || !strings.Contains(report.Repositories[1].Error, "authentication failed") {
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
	for _, order := range []string{"asc", "desc"} {
		got := best(items, Options{Sort: "created", Order: order, Limit: 2})
		if len(got) != 2 || got[0].URL != "a1" || got[1].URL != "a2" {
			t.Errorf("best(%q) = %+v", order, got)
		}
	}
}

func TestControls(t *testing.T) {
	for state, sort := range map[string]string{"open": "created", "closed": "closed", "all": "updated", "merged": "merged"} {
		o := Options{Resource: "prs", State: state, Order: "desc", Limit: 15}
		if err := o.Validate(); err != nil || o.Sort != sort {
			t.Errorf("Validate(%q) = %+v, %v", state, o, err)
		}
	}
	for _, o := range []Options{
		{Resource: "issues", State: "merged", Sort: "merged", Order: "desc", Limit: 15},
		{Resource: "issues", State: "open", Sort: "closed", Order: "desc", Limit: 15},
		{Resource: "prs", State: "closed", Sort: "merged", Order: "desc", Limit: 15},
		{Resource: "issues", State: "open", Sort: "created", Order: "invalid", Limit: 15},
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
	items, err := decode(payload(t, rows), "github.com/a/b", Options{Resource: "prs", State: "closed", Sort: "closed"})
	if err != nil || len(items) != 2 || items[0].State != "merged" || items[1].State != "closed" || items[1].MergedAt != nil {
		t.Fatalf("decode = %+v, %v", items, err)
	}
}
