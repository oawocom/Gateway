import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import * as XLSX from "xlsx";
import { Ic } from "./Icons";

// Universal report table (doc §11): column-click sorting, optional search,
// pagination and one-click Excel (.xlsx) export. Rendering stays custom per
// column (colors, links); sorting/export use the raw `val`.
export interface Col<T> {
  key: string;
  label: string;
  val: (r: T) => string | number | null | undefined; // sort + export value
  render?: (r: T) => ReactNode;                      // cell UI (defaults to val)
  align?: "right";
}

export default function DataTable<T>({
  cols, rows, exportName, searchHint, searchVal, pageSize = 25, onRowClick, rowTitle,
}: {
  cols: Col<T>[];
  rows: T[];
  exportName: string;                 // "debitor-borclari" → debitor-borclari.xlsx
  searchHint?: string;                // when set, a search box is shown
  searchVal?: (r: T) => string;       // haystack; defaults to all col vals
  pageSize?: number;
  onRowClick?: (r: T) => void;
  rowTitle?: string;
}) {
  const [q, setQ] = useState("");
  const [sortKey, setSortKey] = useState("");
  const [asc, setAsc] = useState(false);
  const [page, setPage] = useState(0);
  const [size, setSize] = useState(pageSize);

  const filtered = useMemo(() => {
    let out = rows;
    if (searchHint && q.trim()) {
      const n = q.trim().toLowerCase();
      const hay = searchVal ?? ((r: T) => cols.map((c) => String(c.val(r) ?? "")).join(" "));
      out = out.filter((r) => hay(r).toLowerCase().includes(n));
    }
    if (sortKey) {
      const col = cols.find((c) => c.key === sortKey);
      if (col) {
        out = [...out].sort((a, b) => {
          const va = col.val(a) ?? "", vb = col.val(b) ?? "";
          const cmp = typeof va === "number" && typeof vb === "number"
            ? va - vb
            : String(va).localeCompare(String(vb), "az");
          return asc ? cmp : -cmp;
        });
      }
    }
    return out;
  }, [rows, q, sortKey, asc, cols, searchHint, searchVal]);

  const pages = Math.max(1, Math.ceil(filtered.length / size));
  const cur = Math.min(page, pages - 1);
  const view = filtered.slice(cur * size, cur * size + size);

  function sortBy(key: string) {
    if (sortKey === key) setAsc(!asc);
    else { setSortKey(key); setAsc(false); }
    setPage(0);
  }

  function exportXlsx() {
    const data = filtered.map((r) => {
      const o: Record<string, string | number> = {};
      for (const c of cols) o[c.label] = c.val(r) ?? "";
      return o;
    });
    const ws = XLSX.utils.json_to_sheet(data);
    const wb = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(wb, ws, "Hesabat");
    XLSX.writeFile(wb, `${exportName}.xlsx`);
  }

  return (
    <>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: ".6rem 1.2rem", gap: ".8rem", flexWrap: "wrap" }}>
        <div style={{ display: "flex", gap: ".6rem", alignItems: "center" }}>
          {searchHint && (
            <input placeholder={searchHint} value={q}
              onChange={(e) => { setQ(e.target.value); setPage(0); }}
              style={{ padding: ".45rem .7rem", border: "1px solid #dfe2ea", width: 250 }} />
          )}
          <span style={{ fontSize: ".8rem", color: "#8b90a3" }}>{filtered.length} sətir</span>
        </div>
        <button onClick={exportXlsx} title="Excel-ə export">{Ic.db} Excel</button>
      </div>
      <table className="tbl" style={{ marginTop: 0, boxShadow: "none" }}>
        <thead><tr>
          {cols.map((c) => (
            <th key={c.key} onClick={() => sortBy(c.key)}
              style={{ cursor: "pointer", userSelect: "none", textAlign: c.align, whiteSpace: "nowrap" }}>
              {c.label} {sortKey === c.key ? (asc ? "▲" : "▼") : <span style={{ color: "#c9ccd8" }}>⇅</span>}
            </th>
          ))}
        </tr></thead>
        <tbody>
          {view.map((r, i) => (
            <tr key={i} title={rowTitle}
              style={onRowClick ? { cursor: "pointer" } : undefined}
              onClick={onRowClick ? () => onRowClick(r) : undefined}>
              {cols.map((c) => (
                <td key={c.key} style={{ textAlign: c.align }}>{c.render ? c.render(r) : c.val(r) ?? "—"}</td>
              ))}
            </tr>
          ))}
          {!view.length && <tr><td colSpan={cols.length} style={{ color: "#888" }}>Məlumat yoxdur</td></tr>}
        </tbody>
      </table>
      {pages > 1 && (
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: ".6rem 1.2rem" }}>
          <span style={{ fontSize: ".82rem", color: "#8b90a3" }}>
            Səhifə {cur + 1} / {pages}
          </span>
          <div style={{ display: "flex", gap: ".4rem", alignItems: "center" }}>
            <select value={size} onChange={(e) => { setSize(Number(e.target.value)); setPage(0); }}
              style={{ padding: ".3rem .5rem", border: "1px solid #dfe2ea" }}>
              {[25, 50, 100].map((n) => <option key={n} value={n}>{n} sətir</option>)}
            </select>
            <button disabled={cur === 0} onClick={() => setPage(cur - 1)}>‹ Əvvəlki</button>
            <button disabled={cur >= pages - 1} onClick={() => setPage(cur + 1)}>Sonrakı ›</button>
          </div>
        </div>
      )}
    </>
  );
}
