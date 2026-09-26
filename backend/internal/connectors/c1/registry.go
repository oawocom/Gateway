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

// ReportDef describes one report a configuration family provides.
// The frontend builds the reports menu from these — nothing is hardcoded.
type ReportDef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Group string `json:"group,omitempty"` // "" = top level; "Maliyyə", "Müştərilər", ...
	Path  string `json:"path"`            // frontend route
}

type family struct {
	name    string
	match   func(p *c1meta.Profile) bool
	create  Constructor
	reports []ReportDef
}

var families []family

// Register adds a configuration family. Called from adapter packages' init().
func Register(name string, match func(*c1meta.Profile) bool, create Constructor, reports []ReportDef) {
	families = append(families, family{name, match, create, reports})
}

// Match returns the family name and its report list for a profile without
// opening the base. ok=false → no adapter supports this configuration.
func Match(p *c1meta.Profile) (string, []ReportDef, bool) {
	for _, f := range families {
		if f.match(p) {
			return f.name, f.reports, true
		}
	}
	return "", nil, false
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
