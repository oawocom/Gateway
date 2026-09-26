import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";
import DataTable from "../../components/DataTable";
import type { Col } from "../../components/DataTable";

interface Conn { id: string; name: string; connector_type: string; source?: string }
interface MRow { year: number; month: number; invoiced: number; paid: number; gap: number; collection_pct: number | null }
interface Pay {
  period: { from: string; to: string };
  kpi: { invoiced: number; paid: number; gap: number; paid_non_invoiced: number; collection_pct?: number };
  monthly: MRow[];
}

const azn = (v: number) => new Intl.NumberFormat("az", { maximumFractionDigits: 0 }).format(v) + " ₼";
const pct = (v: number) => v.toFixed(1).replace(".", ",") + " %";
const iso = (d: Date) => d.toISOString().slice(0, 10);
const MON = ["Yan", "Fev", "Mar", "Apr", "May", "İyn", "İyl", "Avq", "Sen", "Okt", "Noy", "Dek"];
const GOLD = "#f0c000", GRAY = "#8a8fa3", RED = "#c62828", OK = "#16a34a";

function preset(kind: string): [string, string] {
  const now = new Date();
  const first = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
  const add = (d: Date, m: number) => new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + m, 1));
  switch (kind) {
    case "6m": return [iso(add(first, -5)), iso(add(first, 1))];
    case "year": return [iso(new Date(Date.UTC(now.getUTCFullYear(), 0, 1))), iso(add(first, 1))];
    default: return [iso(add(first, -11)), iso(add(first, 1))];
  }
}

function Bars({ rows }: { rows: MRow[] }) {
  if (!rows.length) return <p className="page-sub" style={{ padding: "1rem 0" }}>Məlumat yoxdur</p>;
  const w = 760, h = 250, pad = 48;
  const max = Math.max(...rows.map((r) => Math.max(r.invoiced, r.paid)), 1);
  const gw = (w - pad * 2) / rows.length;
  const bw = Math.min(gw * 0.3, 26);
  const y = (v: number) => h - pad - ((h - pad * 2) * v) / max;
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: w }}>
      <line x1={pad} y1={h - pad} x2={w - pad} y2={h - pad} stroke="#e4e6ee" />
      <text x={pad - 6} y={pad + 4} textAnchor="end" fontSize="10" fill="#888">{azn(max)}</text>
      {rows.map((r, i) => {
        const cx = pad + i * gw + gw / 2;
        return (
          <g key={`${r.year}-${r.month}`}>
            <title>{MON[r.month - 1]} {r.year}: hesablanmış {azn(r.invoiced)}, ödənilmiş {azn(r.paid)}{r.collection_pct != null ? `, yığım ${pct(r.collection_pct)}` : ""}</title>
            <rect x={cx - bw - 1.5} y={y(r.invoiced)} width={bw} height={h - pad - y(r.invoiced)} fill={GRAY} />
            <rect x={cx + 1.5} y={y(r.paid)} width={bw} height={h - pad - y(r.paid)} fill={GOLD} />
            {r.collection_pct != null && (
              <text x={cx} y={Math.min(y(r.invoiced), y(r.paid)) - 6} textAnchor="middle" fontSize="9"
                fill={r.collection_pct >= 90 ? OK : r.collection_pct < 70 ? RED : "#555"} fontWeight="600">
                {Math.round(r.collection_pct)}%
              </text>
            )}
            <text x={cx} y={h - pad + 14} textAnchor="middle" fontSize="10" fill="#555">{MON[r.month - 1]}</text>
            {(i === 0 || r.month === 1) && <text x={cx} y={h - pad + 26} textAnchor="middle" fontSize="9" fill="#999">{r.year}</text>}
          </g>
        );
      })}
      <g>
        <rect x={pad} y={8} width={10} height={10} fill={GRAY} /><text x={pad + 15} y={17} fontSize="10">Hesablanmış (ƏDV daxil)</text>
        <rect x={pad + 165} y={8} width={10} height={10} fill={GOLD} /><text x={pad + 180} y={17} fontSize="10">Ödənilmiş</text>
      </g>
    </svg>
  );
}

function Kpi({ label, value, sub, subColor }: { label: string; value: string; sub?: string; subColor?: string }) {
  return (
    <div className="card">
      <span>{label}</span>
      <strong style={{ fontSize: "1.3rem" }}>{value}</strong>
      {sub && <small style={{ color: subColor ?? "#8b90a3", fontWeight: 600 }}>{sub}</small>}
    </div>
  );
}

export default function Payments1C() {
  const conns = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await api.get<{ connections: Conn[] }>("/connections")).data.connections,
  });
  const c1conns = useMemo(() => (conns.data ?? []).filter((c) => c.connector_type === "mssql"), [conns.data]);
  const [connID, setConnID] = useState("");
  const [sp] = useSearchParams();
  const urlConn = sp.get("connection") ?? "";
  const conn = connID || urlConn || c1conns[0]?.id || "";

  const [kind, setKind] = useState("12m");
  const [[from, to], setRange] = useState<[string, string]>(preset("12m"));

  const { data: d, isLoading, error } = useQuery({
    queryKey: ["c1pay", conn, from, to],
    queryFn: async () => (await api.get<Pay>("/reports/c1/payments", {
      params: { connection: conn, from, to },
    })).data,
    enabled: !!conn, staleTime: 5 * 60 * 1000, retry: false,
  });

  const apply = (kk: string) => { setKind(kk); setRange(preset(kk)); };
  const err = (error as any)?.response?.data?.error ?? (error as any)?.message;
  const k = d?.kpi;

  if (!conns.isLoading && !c1conns.length) {
    return (<><h1>Ödənişlər / Yığım</h1>
      <div className="empty-state"><h2>1C bağlantısı yoxdur</h2><p>İnteqrasiyalar bölməsindən 1C bazanızı qoşun.</p></div></>);
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Ödənişlər / Yığım</h1>
          <p className="page-sub">Hesablanmış vs ödənilmiş məbləğ və yığım faizi — 1C-dən canlı</p>
        </div>
        {c1conns.length > 1 && (
          <select value={conn} onChange={(e) => setConnID(e.target.value)} style={{ padding: ".5rem .7rem", border: "1px solid #dfe2ea" }}>
            {c1conns.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
        )}
      </div>

      <div className="panel" style={{ marginTop: "1rem" }}>
        <div className="panel-head" style={{ flexWrap: "wrap", gap: ".6rem" }}>
          <div className="chips" style={{ marginTop: 0 }}>
            {[["6m", "Son 6 ay"], ["12m", "Son 12 ay"], ["year", "Bu il"]].map(([kk, l]) => (
              <button key={kk} className={"chip" + (kind === kk ? " on" : "")} onClick={() => apply(kk)}>{l}</button>
            ))}
          </div>
          <div style={{ display: "flex", gap: ".5rem", alignItems: "center", fontSize: ".85rem" }}>
            <input type="date" value={from} onChange={(e) => { setKind("custom"); setRange([e.target.value, to]); }} style={{ padding: ".4rem .6rem", border: "1px solid #dfe2ea" }} />
            <span>—</span>
            <input type="date" value={to} onChange={(e) => { setKind("custom"); setRange([from, e.target.value]); }} style={{ padding: ".4rem .6rem", border: "1px solid #dfe2ea" }} />
          </div>
        </div>
      </div>

      {err && <p className="error" style={{ marginTop: "1rem" }}>{err}</p>}
      {isLoading && <p style={{ marginTop: "1rem" }}>1C-dən hesablanır…</p>}

      {d && k && (
        <>
          <div className="cards" style={{ marginTop: "1.2rem" }}>
            <Kpi label="Hesablanmış (ƏDV daxil)" value={azn(k.invoiced)} sub={`${d.period.from} — ${d.period.to}`} />
            <Kpi label="Ödənilmiş" value={azn(k.paid)}
              sub={k.collection_pct !== undefined ? `yığım faizi ${pct(k.collection_pct)}` : undefined}
              subColor={k.collection_pct !== undefined && k.collection_pct >= 90 ? OK : undefined} />
            <Kpi label="Fərq (yığılmamış)" value={azn(k.gap)} subColor={k.gap > 0 ? RED : OK}
              sub={k.gap > 0 ? "hesablanmışdan az ödənilib" : "tam yığılıb"} />
            <Kpi label="Qeyri-invoys ödəyicilər" value={azn(k.paid_non_invoiced)}
              sub="invoice-u olmayan ödəyicilərdən daxilolma (aqreqatorlar və s.)" />
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.activity} Hesablanmış vs ödənilmiş — aylar üzrə</h2></div>
            <div style={{ padding: "12px 16px" }}><Bars rows={d.monthly} /></div>
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.db} Aylıq cədvəl</h2></div>
            <DataTable rows={d.monthly} exportName="odenisler-yigim" pageSize={50}
              cols={[
                { key: "ay", label: "Ay", val: (r) => r.year * 100 + r.month, render: (r) => `${MON[r.month - 1]} ${r.year}` },
                { key: "inv", label: "Hesablanmış", val: (r) => r.invoiced, render: (r) => azn(r.invoiced), align: "right" },
                { key: "paid", label: "Ödənilmiş", val: (r) => r.paid, render: (r) => azn(r.paid), align: "right" },
                { key: "gap", label: "Fərq", val: (r) => r.gap, align: "right",
                  render: (r) => <span style={{ color: r.gap > 0 ? RED : OK }}>{azn(r.gap)}</span> },
                { key: "col", label: "Yığım faizi", val: (r) => r.collection_pct ?? 0, align: "right",
                  render: (r) => <span style={{ fontWeight: 600, color: r.collection_pct == null ? "#8b90a3" : r.collection_pct >= 90 ? OK : r.collection_pct < 70 ? RED : undefined }}>
                    {r.collection_pct == null ? "—" : pct(r.collection_pct)}</span> },
              ] satisfies Col<MRow>[]} />
          </div>
        </>
      )}
    </>
  );
}
