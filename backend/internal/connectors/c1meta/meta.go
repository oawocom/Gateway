// Package c1meta reads a 1C:Enterprise 8.x infobase's own metadata from its
// MSSQL storage and resolves metadata names (Справочник.Контрагенты) to
// physical table names (_Reference62). No table numbers are ever hardcoded:
// everything comes from Params.DBNames + Config + IBVersion of the base.
//
// Read-only: only SELECT statements are issued.
package c1meta

import (
	"bytes"
	"compress/flate"
	"database/sql"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Profile identifies the 1C base and holds the resolved name→table map.
type Profile struct {
	Platform      string `json:"platform"`       // e.g. "8.2.14" (format the base requires)
	ConfigName    string `json:"config_name"`    // e.g. "AzStandart"
	ConfigSynonym string `json:"config_synonym"` // e.g. "Бухгалтерия предприятия для Азербайджана, редакция 1.0"
	ConfigVersion string `json:"config_version"` // e.g. "1.1.2.6"
	Vendor        string `json:"vendor"`         // e.g. "COMPLEX SERVİCES MMC"
	ConfigGUID    string `json:"config_guid"`

	// Tables: "Справочник.Контрагенты" -> "_Reference62"
	Tables map[string]string `json:"tables"`
	// Fields: "Документ.РеализацияТоваровУслуг.Контрагент" -> ["_Fld5388"]
	// (several candidates when the same attribute name exists in header and
	// tabular sections; Resolve() picks the one present in the target table)
	Fields map[string][]string `json:"fields"`
	// TabularSections: "Документ.РеализацияТоваровУслуг.Товары" -> "_Document209_VT5413"
	TabularSections map[string]string `json:"tabular_sections"`
	// AccRgED: register table -> its extra-dimensions (subconto) table
	AccRgED map[string]string `json:"accrg_ed"`
}

var (
	reGUID      = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	reDBName    = regexp.MustCompile(`\{([0-9a-f-]{36}),"([A-Za-z]+)",([0-9]+)\}`)
	reObjName   = regexp.MustCompile(`\{0,0,([0-9a-f-]{36})\},"([^"]*)"`)
	reSynonym   = regexp.MustCompile(`\{2,"ru","((?:[^"]|"")*)"`)
	reVendorVer = regexp.MustCompile(`"((?:[^"]|"")+)","([0-9]+\.[0-9]+\.[0-9]+(?:\.[0-9]+)?)"`)
)

// kindPrefix maps DBNames kinds to 1C metadata class names.
var kindPrefix = map[string]string{
	"Reference": "Справочник",
	"Document":  "Документ",
	"Acc":       "ПланСчетов",
	"AccRg":     "РегистрБухгалтерии",
	"AccumRg":   "РегистрНакопления",
	"InfoRg":    "РегистрСведений",
	"Enum":      "Перечисление",
	"Chrc":      "ПланВидовХарактеристик",
	"Const":     "Константа",
	"Task":      "Задача",
	"BPr":       "БизнесПроцесс",
}

type dbName struct {
	kind string
	num  int
}

// Load reads the base's identity and metadata map. The *sql.DB must point at
// the 1C infobase (MSSQL).
func Load(db *sql.DB) (*Profile, error) {
	p := &Profile{
		Tables:          map[string]string{},
		Fields:          map[string][]string{},
		TabularSections: map[string]string{},
		AccRgED:         map[string]string{},
	}

	// 1) platform format version
	var req int
	if err := db.QueryRow(`SELECT TOP 1 PlatformVersionReq FROM IBVersion`).Scan(&req); err != nil {
		return nil, fmt.Errorf("IBVersion: %w", err)
	}
	p.Platform = fmt.Sprintf("%d.%d.%d", req/10000, (req/100)%100, req%100)

	// 2) config root -> config GUID
	root, err := readFile(db, "Config", "root")
	if err != nil {
		return nil, fmt.Errorf("Config.root: %w", err)
	}
	p.ConfigGUID = reGUID.FindString(root)
	if p.ConfigGUID == "" {
		return nil, fmt.Errorf("Config.root: config GUID not found")
	}

	// 3) config header -> name, synonym, vendor, version
	hdr, err := readFile(db, "Config", p.ConfigGUID)
	if err != nil {
		return nil, fmt.Errorf("Config[%s]: %w", p.ConfigGUID, err)
	}
	if m := reObjName.FindStringSubmatch(hdr); m != nil {
		p.ConfigName = m[2]
	}
	if m := reSynonym.FindStringSubmatch(hdr); m != nil {
		p.ConfigSynonym = unq(m[1])
	}
	if m := reVendorVer.FindStringSubmatch(hdr); m != nil {
		p.Vendor = unq(m[1])
		p.ConfigVersion = m[2]
	}

	// 4) DBNames -> GUID -> (kind, table number)
	dbn, err := readFile(db, "Params", "DBNames")
	if err != nil {
		return nil, fmt.Errorf("Params.DBNames: %w", err)
	}
	// one GUID may carry several entries (e.g. a register's AccRg + AccRgED)
	all := map[string][]dbName{}
	for _, m := range reDBName.FindAllStringSubmatch(dbn, -1) {
		n, _ := strconv.Atoi(m[3])
		all[m[1]] = append(all[m[1]], dbName{kind: m[2], num: n})
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("Params.DBNames: empty")
	}
	// primary entry per GUID: the one whose kind is a metadata class, else the first
	names := map[string]dbName{}
	for g, list := range all {
		names[g] = list[0]
		for _, d := range list {
			if _, ok := kindPrefix[d.kind]; ok {
				names[g] = d
				break
			}
		}
		// register -> its extra-dimensions table (same GUID in DBNames)
		var rg, ed int
		for _, d := range list {
			if d.kind == "AccRg" {
				rg = d.num
			}
			if d.kind == "AccRgED" {
				ed = d.num
			}
		}
		if rg > 0 && ed > 0 {
			p.AccRgED[fmt.Sprintf("_AccRg%d", rg)] = fmt.Sprintf("_AccRgED%d", ed)
		}
	}

	// 5) object names: read Config file for every top-level object GUID
	var objGUIDs []string
	for g, d := range names {
		if _, ok := kindPrefix[d.kind]; ok {
			objGUIDs = append(objGUIDs, g)
		}
	}
	sort.Strings(objGUIDs)
	const batch = 150
	for i := 0; i < len(objGUIDs); i += batch {
		j := i + batch
		if j > len(objGUIDs) {
			j = len(objGUIDs)
		}
		files, err := readFiles(db, "Config", objGUIDs[i:j])
		if err != nil {
			return nil, fmt.Errorf("Config objects: %w", err)
		}
		for g, txt := range files {
			obj := names[g]
			pairs := reObjName.FindAllStringSubmatch(txt, -1)
			if len(pairs) == 0 {
				continue
			}
			// first {0,0,GUID},"Name" pair whose GUID is the object itself
			objName := ""
			for _, pr := range pairs {
				if pr[1] == g {
					objName = pr[2]
					break
				}
			}
			if objName == "" {
				objName = pairs[0][2]
			}
			full := kindPrefix[obj.kind] + "." + objName
			table := fmt.Sprintf("_%s%d", obj.kind, obj.num)
			p.Tables[full] = table
			// children: attributes (Fld) and tabular sections (VT)
			for _, pr := range pairs {
				if pr[1] == g {
					continue
				}
				child, ok := names[pr[1]]
				if !ok {
					continue
				}
				switch child.kind {
				case "Fld":
					key := full + "." + pr[2]
					p.Fields[key] = append(p.Fields[key], fmt.Sprintf("_Fld%d", child.num))
				case "VT":
					p.TabularSections[full+"."+pr[2]] = fmt.Sprintf("%s_VT%d", table, child.num)
				}
			}
		}
	}
	return p, nil
}

// Table returns the physical table for a metadata name, or "".
func (p *Profile) Table(metaName string) string { return p.Tables[metaName] }

// Resolve returns the physical column (with its real suffix, e.g. _Fld5388RRef)
// for object.attribute inside table, checking sys.columns of the base.
func (p *Profile) Resolve(db *sql.DB, table, object, attr string) (string, error) {
	cands := p.Fields[object+"."+attr]
	if len(cands) == 0 {
		return "", fmt.Errorf("%s.%s: attribute not in metadata", object, attr)
	}
	for _, c := range cands {
		var col string
		err := db.QueryRow(`SELECT TOP 1 name FROM sys.columns WHERE object_id=OBJECT_ID(@p1) AND (name=@p2 OR name=@p2+'RRef' OR name=@p2+'_RRRef') ORDER BY name`, table, c).Scan(&col)
		if err == nil && col != "" {
			return col, nil
		}
	}
	return "", fmt.Errorf("%s.%s: no candidate column in %s", object, attr, table)
}

// ---------- storage helpers ----------

func readFile(db *sql.DB, table, name string) (string, error) {
	files, err := readFiles(db, table, []string{name})
	if err != nil {
		return "", err
	}
	txt, ok := files[name]
	if !ok {
		return "", fmt.Errorf("%s.%s not found", table, name)
	}
	return txt, nil
}

// readFiles fetches and inflates several 1C storage files in one query.
func readFiles(db *sql.DB, table string, names []string) (map[string]string, error) {
	if table != "Config" && table != "Params" {
		return nil, fmt.Errorf("bad table")
	}
	args := make([]any, len(names))
	ph := make([]string, len(names))
	for i, n := range names {
		args[i] = n
		ph[i] = fmt.Sprintf("@p%d", i+1)
	}
	q := fmt.Sprintf(`SELECT FileName, CAST(BinaryData AS varbinary(max)) FROM %s WHERE FileName IN (%s)`, table, strings.Join(ph, ","))
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name string
		var raw []byte
		if err := rows.Scan(&name, &raw); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(name)] = inflate(raw)
	}
	return out, rows.Err()
}

// inflate decompresses raw-deflate 1C storage; falls back to raw bytes.
func inflate(raw []byte) string {
	r := flate.NewReader(bytes.NewReader(raw))
	b, err := io.ReadAll(r)
	r.Close()
	if err != nil || len(b) == 0 {
		b = raw
	}
	return strings.TrimPrefix(string(b), "\ufeff")
}

func unq(s string) string { return strings.ReplaceAll(s, `""`, `"`) }
