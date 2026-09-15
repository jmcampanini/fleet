package repos

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jmcampanini/fleet/internal/inventory"
)

func TestListDescribesInventoryAndSortsTopics(t *testing.T) {
	var calls []string
	var record sync.Mutex
	client := Client{Run: func(_ context.Context, dir, program string, args ...string) (string, error) {
		record.Lock()
		calls = append(calls, program+" "+strings.Join(args, " "))
		record.Unlock()
		if dir != "" || program != "gh" {
			t.Fatalf("Run(%q, %q, %v)", dir, program, args)
		}
		if args[2] == "github.com/a/two" {
			return `{"repositoryTopics":null}`, nil
		}
		return `{"repositoryTopics":[{"name":"go"},{"name":"cli"}]}`, nil
	}}
	repos := []inventory.Repository{{ID: "github.com/a/one", Branch: "develop", Groups: []string{"clis"}}, {ID: "github.com/a/two"}}

	report := client.List(t.Context(), "/code", repos, true)

	want := Report{Complete: true, TopicsFetched: true, Results: []Entry{
		{Repository: "github.com/a/one", Path: "/code/github.com/a/one", Branch: "develop", Groups: []string{"clis"}, Topics: []string{"cli", "go"}},
		{Repository: "github.com/a/two", Path: "/code/github.com/a/two", Groups: []string{}, Topics: []string{}},
	}}
	if !reflect.DeepEqual(report, want) {
		t.Errorf("List() = %+v, want %+v", report, want)
	}
	sort.Strings(calls)
	if !reflect.DeepEqual(calls, []string{"gh repo view github.com/a/one --json repositoryTopics", "gh repo view github.com/a/two --json repositoryTopics"}) {
		t.Errorf("gh calls = %v", calls)
	}
}

func TestListWithoutTopicsNeverRunsGh(t *testing.T) {
	client := Client{Run: func(_ context.Context, _, program string, args ...string) (string, error) {
		t.Fatalf("unexpected %s %v", program, args)
		return "", nil
	}}

	report := client.List(t.Context(), "/code", []inventory.Repository{{ID: "github.com/a/one"}}, false)

	if !report.Complete || report.TopicsFetched || len(report.Results) != 1 || len(report.Results[0].Topics) != 0 || report.Results[0].Topics == nil {
		t.Errorf("List() = %+v, want a complete report with an empty topic list", report)
	}
}

func TestListContinuesAfterTopicFailure(t *testing.T) {
	client := Client{Run: func(_ context.Context, _, _ string, args ...string) (string, error) {
		switch args[2] {
		case "github.com/a/fails":
			return "", errors.New("gh: HTTP 401")
		case "github.com/a/garbled":
			return `{"repositoryTopics":[{"name":""}]}`, nil
		}
		return `{"repositoryTopics":[{"name":"go"}]}`, nil
	}}
	repos := []inventory.Repository{{ID: "github.com/a/fails"}, {ID: "github.com/a/garbled"}, {ID: "github.com/a/ok"}}

	report := client.List(t.Context(), "/code", repos, true)

	if report.Complete {
		t.Error("report is complete despite failures")
	}
	failed, garbled, ok := report.Results[0], report.Results[1], report.Results[2]
	for _, want := range []string{`"github.com/a/fails"`, `host "github.com"`, "gh: HTTP 401", "--no-topics"} {
		if !strings.Contains(failed.Error, want) {
			t.Errorf("error %q lacks %q", failed.Error, want)
		}
	}
	if len(failed.Topics) != 0 || failed.Topics == nil {
		t.Errorf("failed entry topics = %v, want an empty list", failed.Topics)
	}
	if garbled.Error == "" || !strings.Contains(garbled.Error, "unnamed topic") {
		t.Errorf("garbled entry error = %q", garbled.Error)
	}
	if ok.Error != "" || !reflect.DeepEqual(ok.Topics, []string{"go"}) {
		t.Errorf("ok entry = %+v", ok)
	}
}
