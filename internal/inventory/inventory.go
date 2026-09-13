// Package inventory validates repository definitions and resolves selections.
package inventory

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Repository is a validated complete identity with its branch override.
type Repository struct {
	Branch string
	ID     string
}

// Inventory owns the complete namespace and resolved group membership.
type Inventory struct {
	groups map[string][]string
	repos  map[string]Repository
}

// New validates every identity and group before any selection is used.
// branches maps each complete identity to its branch override, or to an
// empty string when the remote default branch applies.
func New(branches map[string]string, groups map[string][]string) (Inventory, error) {
	inv := Inventory{groups: make(map[string][]string), repos: make(map[string]Repository)}
	for _, id := range sortedKeys(branches) {
		if err := ValidateIdentity(id); err != nil {
			return Inventory{}, err
		}
		branch := branches[id]
		if branch != "" && !validBranch(branch) {
			return Inventory{}, fmt.Errorf("repository %q: invalid branch %q", id, branch)
		}
		inv.repos[id] = Repository{ID: id, Branch: branch}
	}
	for _, name := range sortedKeys(groups) {
		if name == "" || strings.ContainsAny(name, ",\x00\r\n\t") {
			return Inventory{}, fmt.Errorf("invalid group name %q", name)
		}
		inv.groups[name] = nil
		for _, ref := range groups[name] {
			id, err := inv.resolve(ref)
			if err != nil {
				return Inventory{}, fmt.Errorf("group %q: %w", name, err)
			}
			inv.groups[name] = append(inv.groups[name], id)
		}
	}
	return inv, nil
}

// ValidateIdentity rejects identities that cannot safely become SSH paths.
func ValidateIdentity(id string) error {
	parts := strings.Split(id, "/")
	component := regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	host := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)
	if len(parts) != 3 || !host.MatchString(parts[0]) {
		return fmt.Errorf("invalid repository identity %q: use server/org/repo without a URL scheme", id)
	}
	for _, part := range parts {
		if !component.MatchString(part) || part == "." || part == ".." {
			return fmt.Errorf("invalid repository identity %q: unsafe component %q", id, part)
		}
	}
	return nil
}

func validBranch(branch string) bool {
	if strings.HasPrefix(branch, "-") || strings.ContainsAny(branch, " ~^:?*[\\\x7f") || strings.Contains(branch, "..") || strings.Contains(branch, "@{") || branch == "@" {
		return false
	}
	for _, r := range branch {
		if r < 32 {
			return false
		}
	}
	for _, part := range strings.Split(branch, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") || strings.HasSuffix(part, ".") {
			return false
		}
	}
	return true
}

// Select returns the deduplicated union in complete-identity order.
// An explicit empty group remains an empty selection.
func (inv Inventory) Select(refs, groups []string) ([]Repository, error) {
	selected := make(map[string]bool)
	if len(refs) == 0 && len(groups) == 0 {
		for id := range inv.repos {
			selected[id] = true
		}
	}
	for _, ref := range refs {
		id, err := inv.resolve(ref)
		if err != nil {
			return nil, err
		}
		selected[id] = true
	}
	for _, group := range groups {
		members, ok := inv.groups[group]
		if !ok {
			return nil, fmt.Errorf("unknown group %q", group)
		}
		for _, id := range members {
			selected[id] = true
		}
	}
	var repos []Repository
	for _, id := range sortedKeys(selected) {
		repos = append(repos, inv.repos[id])
	}
	return repos, nil
}

func (inv Inventory) resolve(ref string) (string, error) {
	var matches []string
	for id := range inv.repos {
		if ref != "" && (id == ref || strings.HasSuffix(id, "/"+ref)) {
			matches = append(matches, id)
		}
	}
	slices.Sort(matches)
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("unknown repository %q", ref)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous repository %q: use one of %s", ref, strings.Join(matches, ", "))
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
