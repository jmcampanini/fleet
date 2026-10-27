package query

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/jmcampanini/fleet/internal/inventory"
)

// Key is an item's place in the total order of a validated query: the sort
// timestamp in the requested direction, then repository identity and item
// number, ascending.
type Key struct {
	Time       time.Time `json:"t"`
	Repository string    `json:"r"`
	Number     int       `json:"n"`
}

func (i Item) key(sort Sort) Key {
	return Key{Time: i.Timestamp(sort), Repository: i.Repository, Number: i.Number}
}

// compareKeys is negative when a sorts before b in the given order.
func compareKeys(a, b Key, order Order) int {
	if c := a.Time.Compare(b.Time); c != 0 {
		if order == OrderDesc {
			return -c
		}
		return c
	}
	if c := strings.Compare(a.Repository, b.Repository); c != 0 {
		return c
	}
	return cmp.Compare(a.Number, b.Number)
}

// Cursor records how far earlier pages read each repository: the key of the
// last item already returned from it, or nil when nothing was. After applies
// to every repository that Repositories does not name; a repository whose
// read failed keeps its earlier position there. The zero Cursor starts every
// repository from its first item.
type Cursor struct {
	After        *Key            `json:"a"`
	Query        string          `json:"q"`
	Repositories map[string]*Key `json:"r,omitempty"`
}

// Errors ParseCursor reports.
var (
	ErrCursorInvalid = errors.New("not a nextCursor value")
	ErrCursorQuery   = errors.New("the cursor belongs to a different query; resource, state, sort, order, and the selected repositories must match")
)

// ParseCursor decodes a nextCursor value for a validated query over repos.
func ParseCursor(text string, options Options, repos []inventory.Repository) (Cursor, error) {
	body, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil {
		return Cursor{}, ErrCursorInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var cursor Cursor
	if err := decoder.Decode(&cursor); err != nil || decoder.More() || cursor.Query == "" {
		return Cursor{}, ErrCursorInvalid
	}
	if cursor.Query != fingerprint(options, repos) {
		return Cursor{}, ErrCursorQuery
	}
	for _, key := range append(slices.Collect(maps.Values(cursor.Repositories)), cursor.After) {
		if key != nil && (key.Time.IsZero() || key.Repository == "" || key.Number <= 0) {
			return Cursor{}, ErrCursorInvalid
		}
	}
	return cursor, nil
}

// position returns the key of the last item already returned from repo.
func (c Cursor) position(repo string) *Key {
	if key, ok := c.Repositories[repo]; ok {
		return key
	}
	return c.After
}

// next advances every repository read in report past its last returned
// item, and past its own copies of returned URLs, which deduplication kept
// under another repository. A failed repository stays where c had it, so
// its items return on the next page without repeating the other
// repositories' items.
func (c Cursor) next(repos []inventory.Repository, report Report, candidates [][]Item) Cursor {
	var last *Key
	returned := make(map[string]bool, len(report.Items))
	for _, item := range report.Items {
		returned[item.URL] = true
	}
	if len(report.Items) > 0 {
		key := report.Items[len(report.Items)-1].key(report.Query.Sort)
		last = &key
	}
	next := Cursor{After: later(c.After, last, report.Query.Order)}
	for index, repo := range repos {
		position := c.position(repo.ID)
		if report.Repositories[index].Complete {
			position = later(position, last, report.Query.Order)
			for _, item := range candidates[index] {
				if returned[item.URL] {
					key := item.key(report.Query.Sort)
					position = later(position, &key, report.Query.Order)
				}
			}
		}
		if !sameKey(position, next.After, report.Query.Order) {
			if next.Repositories == nil {
				next.Repositories = make(map[string]*Key)
			}
			next.Repositories[repo.ID] = position
		}
	}
	return next
}

// encode returns the opaque text a later --cursor passes to ParseCursor.
func (c Cursor) encode(options Options, repos []inventory.Repository) string {
	c.Query = fingerprint(options, repos)
	// A Cursor holds only strings, integers, and timestamps, which always
	// marshal.
	body, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(body)
}

// fingerprint identifies the controls and selection a cursor's keys depend
// on. The limit is not part of it, so pages may differ in size.
func fingerprint(options Options, repos []inventory.Repository) string {
	hash := sha256.New()
	for _, part := range []string{string(options.Resource), string(options.State), string(options.Sort), string(options.Order)} {
		hash.Write([]byte(part + "\n"))
	}
	for _, repo := range repos {
		hash.Write([]byte(repo.ID + "\n"))
	}
	return hex.EncodeToString(hash.Sum(nil)[:8])
}

// after reports whether item sorts after position; a nil position precedes
// every item.
func after(item Item, position *Key, options Options) bool {
	return position == nil || compareKeys(item.key(options.Sort), *position, options.Order) > 0
}

// later returns whichever position sorts last; nil sorts first.
func later(a, b *Key, order Order) *Key {
	if a == nil || (b != nil && compareKeys(*b, *a, order) > 0) {
		return b
	}
	return a
}

func sameKey(a, b *Key, order Order) bool {
	if a == nil || b == nil {
		return a == b
	}
	return compareKeys(*a, *b, order) == 0
}
