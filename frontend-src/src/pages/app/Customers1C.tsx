import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";
import DataTable from "../../components/DataTable";
import type { Col } from "../../components/DataTable";

interface Conn { id: string; name: string; connector_type: string; source?: string }
interface MRow { year: number; month: number; new: number; lost: number; net: number }
interface TopRow {
  customer_id: string; customer: string; voen: string;
  invoiced: number; paid: number; invoices: number; share_pct: number; cum_pct: number;
}
interface Cust {
  period: { from: string; to: string };
  kpi: { active: number; total_customers: number; new: number; lost: number; net_growth: number;
         top5_share_pct: number; top10_share_pct: number };
  monthly: MRow[]; top: TopRow[];
}

const azn = (v: number) => new Intl.NumberFormat("az", { maximumFractionDigits: 0 }).format(v) + " ₼";
const pct = (v: number) => v.toFixed(1).replace(".", ",") + " %";
const iso = (d: Date) => d.toISOString().slice(0, 10);
const MON = ["Yan", "Fev", "Mar", "Apr", "May", "İyn", "İyl", "Avq", "Sen", "Okt", "Noy", "Dek"];
const RED = "#c62828", OK = "#16a34a";

function preset(kind: string): [string, string] {
  const now = new Date();
  const first = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
  const add = (d: Date, m: number) => new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + m, 1));
  switch (kind) {
    case "6m": return [iso(add(first, -5)), iso(add(first, 1))];
    case "year": return [iso(new Date(Date.UTC(now.getUTCFullYear(), 0, 1))), iso(add(first, 1))];
    case "24m": return [iso(add(first, -23)), iso(add(first, 1))];
    default: return [iso(add(first, -11)), iso(add(first, 1))];
  }
}

function GrowthBars({ rows }: { rows: MRow[] }) {
  if (!rows.length) return <p className="page-sub" style={{ padding: "1rem 0" }}>Məlumat yoxdur</p>;
  const w = 760, h = 240, pad = 44, mid = h / 2;
  const max = Math.max(...rows.map((r) => Math.max(r.new, r.lost)), 1);
  const gw = (w - pad * 2) / rows.length;
  const bw = Math.min(gw * 0.5, 28);
  const sy = (v: number) => ((mid - pad) * v) / max;
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: w }}>
      <line x1={pad} y1={mid} x2={w - pad} y2={mid} stroke="#e4e6ee" />
      {rows.map((r, i) => {
        const cx = pad + i * gw + gw / 2;
        return (
          <g key={`${r.year}-${r.month}`}>
            <title>{MON[r.month - 1]} {r.year}: +{r.new} yeni, −{r.lost} itirilmiş, net {r.net >= 0 ? "+" : ""}{r.net}</title>
            <rect x={cx - bw / 2} y={mid - sy(r.new)} width={bw} height={sy(r.new)} fill={OK} opacity={0.85} />
            <rect x={cx - bw / 2} y={mid} width={bw} height={sy(r.lost)} fill={RED} opacity={0.8} />
            {r.new > 0 && <text x={cx} y={mid - sy(r.new) - 4} textAnchor="middle" fontSize="9" fill={OK}>+{r.new}</text>}
            {r.lost > 0 && <text x={cx} y={mid + sy(r.lost) + 11} textAnchor="middle" fontSize="9" fill={RED}>−{r.lost}</text>}
            <text x={cx} y={h - 8} textAnchor="middle" fontSize="10" fill="#555">{MON[r.month - 1]}</text>
            {(i === 0 || r.month === 1) && <text x={cx} y={h - 20} textAnchor="middle" fontSize="9" fill="#999">{r.year}</text>}
          </g>
        );
      })}
      <g>
        <rect x={pad} y={8} width={10} height={10} fill={OK} /><text x={pad + 15} y={17} fontSize="10">Yeni müştərilər</text>
        <rect x={pad + 120} y={8} width={10} height={10} fill={RED} /><text x={pad + 135} y={17} fontSize="10">İtirilmiş müştərilər</text>
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

export default function Customers1C() {
  const nav = useNavigate();
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
    queryKey: ["c1cust", conn, from, to],
    queryFn: async () => (await api.get<Cust>("/reports/c1/customers", {
      params: { connection: conn, from, to },
    })).data,
    enabled: !!conn, staleTime: 5 * 60 * 1000, retry: false,
  });

  const apply = (kk: string) => { setKind(kk); setRange(preset(kk)); };
  const err = (error as any)?.response?.data?.error ?? (error as any)?.message;
  const k = d?.kpi;


  if (!conns.isLoading && !c1conns.length) {
    return (<><h1>Müştərilər</h1>
      <div className="empty-state"><h2>1C bağlantısı yoxdur</h2><p>İnteqrasiyalar bölməsindən 1C bazanızı qoşun.</p></div></>);
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Müştərilər / TOP</h1>
          <p className="page-sub">Müştəri bazasının artımı və gəlir konsentrasiyası — 1C-dən canlı</p>
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
            {[["6m", "Son 6 ay"], ["12m", "Son 12 ay"], ["24m", "Son 24 ay"], ["year", "Bu il"]].map(([kk, l]) => (
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
            <Kpi label="Aktiv müştəri (dövr)" value={String(k.active)} sub={`bazada cəmi ${k.total_customers}`} />
            <Kpi label="Net Growth" value={`${k.net_growth >= 0 ? "+" : ""}${k.net_growth}`}
              sub={`+${k.new} yeni · −${k.lost} itirilmiş`} subColor={k.net_growth >= 0 ? OK : RED} />
            <Kpi label="Top 5 payı" value={pct(k.top5_share_pct)}
              sub="ümumi gəlirdə" subColor={k.top5_share_pct > 50 ? RED : undefined} />
            <Kpi label="Top 10 payı" value={pct(k.top10_share_pct)}
              sub="ümumi gəlirdə" subColor={k.top10_share_pct > 70 ? RED : undefined} />
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.activity} Yeni və itirilmiş müştərilər — aylar üzrə</h2>
              <span className="page-sub" style={{ marginTop: 0 }}>itirilmiş = son invoice həmin ayda (son data ayı istisna)</span></div>
            <div style={{ padding: "12px 16px" }}><GrowthBars rows={d.monthly} /></div>
          </div>

          <div className="panel">
            <div className="panel-head">
              <h2>{Ic.users} TOP müştərilər</h2>
              <span className="page-sub" style={{ marginTop: 0 }}>sətrə klik → Customer 360°</span>
            </div>
            <DataTable rows={d.top} exportName="top-musteriler"
              searchHint="Axtar: ad, VÖEN…" searchVal={(r) => `${r.customer} ${r.voen}`}
              rowTitle="Customer 360° aç"
              onRowClick={(r) => nav(`/reports/1c/customer360?customer=${r.customer_id}&connection=${conn}`)}
              cols={[
                { key: "name", label: "Müştəri", val: (r) => r.customer, render: (r) => <b>{r.customer}</b> },
                { key: "voen", label: "VÖEN", val: (r) => r.voen || "—" },
                { key: "inv", label: "Hesablanmış", val: (r) => r.invoiced, render: (r) => azn(r.invoiced), align: "right" },
                { key: "paid", label: "Ödənilmiş", val: (r) => r.paid, render: (r) => azn(r.paid), align: "right" },
                { key: "cnt", label: "İnvoice sayı", val: (r) => r.invoices, align: "right" },
                { key: "share", label: "Gəlirdə pay", val: (r) => r.share_pct, align: "right",
                  render: (r) => <b>{pct(r.share_pct)}</b> },
                { key: "cum", label: "Kumulyativ", val: (r) => r.cum_pct, align: "right",
                  render: (r) => <span style={{ color: "#8b90a3" }}>{pct(r.cum_pct)}</span> },
              ] satisfies Col<TopRow>[]} />
          </div>
        </>
      )}
    </>
  );
}
