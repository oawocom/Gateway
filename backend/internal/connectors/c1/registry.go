package c1

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"gateway/internal/connectors/c1meta"
)

// Constructor builds an adapter over an open 1C base.
type Constructor func(db *sql.DB, p *c1meta.Profile) (Adapter, error)

type family struct {
	name   string
	match  func(p *c1meta.Profile) bool
	create Constructor
}

var families []family

// Register adds a configuration family. Called from adapter packages' init().
func Register(name string, match func(*c1meta.Profile) bool, create Constructor) {
	families = append(families, family{name, match, create})
}

// Select picks the adapter for a profile.
func Select(db *sql.DB, p *c1meta.Profile) (Adapter, error) {
	for _, f := range families {
		if f.match(p) {
			return f.create(db, p)
		}
	}
	return nil, fmt.Errorf("dəstəklənməyən 1C konfiqurasiyası: %s %s (%s)", p.ConfigName, p.ConfigVersion, p.ConfigSynonym)
}

// MajorVersion returns the first number of "1.1.2.6" → 1; 0 if unparsable.
func MajorVersion(v string) int {
	n, _ := strconv.Atoi(strings.SplitN(v, ".", 2)[0])
	return n
}
