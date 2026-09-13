package inventory

import (
	"reflect"
	"strings"
	"testing"
)

func TestSelectionUnion(t *testing.T) {
	inv, err := New(map[string]string{"github.com/a/one": "", "github.com/a/two": "", "other.example/a/three": ""}, map[string][]string{"first": {"one", "a/two"}, "overlap": {"github.com/a/one", "three"}, "empty": {}})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name               string
		refs, groups, want []string
	}{
		{name: "all", want: []string{"github.com/a/one", "github.com/a/two", "other.example/a/three"}},
		{name: "mixed union", refs: []string{"one", "a/two"}, groups: []string{"first", "overlap"}, want: []string{"github.com/a/one", "github.com/a/two", "other.example/a/three"}},
		{name: "empty", groups: []string{"empty"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repos, err := inv.Select(tt.refs, tt.groups)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, repo := range repos {
				got = append(got, repo.ID)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Select() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAmbiguityUsesWholeInventory(t *testing.T) {
	inv, err := New(map[string]string{"github.com/a/repo": "", "github.com/b/repo": "", "other.example/a/repo": ""}, map[string][]string{"a": {"github.com/a/repo"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"repo", "a/repo"} {
		_, err := inv.Select([]string{ref}, []string{"a"})
		if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "other.example/a/repo") {
			t.Errorf("Select(%q) error = %v, want complete ambiguity candidates", ref, err)
		}
	}
	if repos, err := inv.Select([]string{"github.com/a/repo"}, nil); err != nil || len(repos) != 1 {
		t.Fatalf("qualified selection = %v, %v", repos, err)
	}
}

func TestInvalidInventoryAndSelection(t *testing.T) {
	if _, err := New(map[string]string{"github.com/example/.github": ""}, nil); err != nil {
		t.Fatalf("valid dot-prefixed repository was rejected: %v", err)
	}
	for _, id := range []string{"https://github.com/a/b", "github.com/a/../b", "github.com//b", "github.com/../b", "-host/a/b", "github.com/a/b,other", "github.com/a/b\n"} {
		if _, err := New(map[string]string{id: ""}, nil); err == nil {
			t.Errorf("New(%q) succeeded", id)
		}
	}
	if _, err := New(map[string]string{"github.com/a/b": ""}, map[string][]string{"unused": {"unknown"}}); err == nil {
		t.Error("unresolved unused group was accepted")
	}
	inv, err := New(map[string]string{"github.com/a/b": ""}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := inv.Select([]string{"missing"}, nil); err == nil {
		t.Error("unknown repository was accepted")
	}
	if _, err := inv.Select(nil, []string{"missing"}); err == nil {
		t.Error("unknown group was accepted")
	}
}
