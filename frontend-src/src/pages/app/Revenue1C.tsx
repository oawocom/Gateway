import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";
import DataTable from "../../components/DataTable";
import type { Col } from "../../components/DataTable";

interface Conn { id: string; name: string; connector_type: string; source?: string }
interface Ref { id: string; name: string }
interface YearRow { year: number; net: number; customers: number }
interface MonthNet { year: number; month: number; net: number }
interface SvcRow {
  service_id: string; service: string; category: string; net: number; vat: number;
  customers: number; lines: number; prev_net: number; change_pct: number | null;
  share_pct: number; avg_per_customer: number;
}
interface Rev {
  period: { from: string; to: string };
  kpi: { net: number; prev_net: number; yoy_net: number; active_customers: number;
         period_growth_pct?: number; yoy_pct?: number; mom_pct?: number; avg_per_customer?: number };
  years: YearRow[]; monthly: MonthNet[]; services: SvcRow[];
  options: { services: Ref[]; managers: Ref[] };
}

const azn = (v: number) => new Intl.NumberFormat("az", { maximumFractionDigits: 0 }).format(v) + " ₼";
const pct = (v: number) => v.toFixed(1).replace(".", ",") + " %";
const iso = (d: Date) => d.toISOString().slice(0, 10);
const GOLD = "#f0c000", GRAY = "#e4e6ee", RED = "#c62828", OK = "#16a34a";
const DONUT = ["#f0c000", "#8a8fa3", "#c9a227", "#4a4f63", "#e2cf7a", "#b3b7c6"];

function preset(kind: string): [string, string] {
  const now = new Date();
  const first = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
  const add = (d: Date, m: number) => new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + m, 1));
  switch (kind) {
    case "month": return [iso(first), iso(add(first, 1))];
    case "quarter": return [iso(add(first, -2)), iso(add(first, 1))];
    case "year": return [iso(new Date(Date.UTC(now.getUTCFullYear(), 0, 1))), iso(add(first, 1))];
    default: return [iso(add(first, -11)), iso(add(first, 1))];
  }
}

function YearBars({ rows, onPick }: { rows: YearRow[]; onPick: (y: number) => void }) {
  if (!rows.length) return <p className="page-sub" style={{ padding: "1rem 0" }}>Məlumat yoxdur</p>;
  const w = 720, h = 230, pad = 46;
  const max = Math.max(...rows.map((r) => r.net), 1);
  const gw = (w - pad * 2) / rows.length;
  const bw = Math.min(gw * 0.55, 70);
  const y = (v: number) => h - pad - ((h - pad * 2) * v) / max;
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: w }}>
      <line x1={pad} y1={h - pad} x2={w - pad} y2={h - pad} stroke={GRAY} />
      <text x={pad - 6} y={pad + 4} textAnchor="end" fontSize="10" fill="#888">{azn(max)}</text>
      {rows.map((r, i) => {
        const cx = pad + i * gw + gw / 2;
        const prev = i > 0 ? rows[i - 1].net : 0;
        const ch = prev ? ((r.net - prev) / prev) * 100 : null;
        return (
          <g key={r.year} style={{ cursor: "pointer" }} onClick={() => onPick(r.year)}>
            <title>{r.year}: {azn(r.net)}{ch !== null ? ` (${ch >= 0 ? "+" : ""}${ch.toFixed(1)}% əvvəlki ilə)` : ""} · {r.customers} müştəri</title>
            <rect x={cx - bw / 2} y={y(r.net)} width={bw} height={h - pad - y(r.net)} fill={GOLD} />
            <text x={cx} y={y(r.net) - 6} textAnchor="middle" fontSize="10" fontWeight="600" fill="#111">{azn(r.net)}</text>
            <text x={cx} y={h - pad + 14} textAnchor="middle" fontSize="11" fill="#555">{r.year}</text>
            {ch !== null && (
              <text x={cx} y={h - pad + 27} textAnchor="middle" fontSize="9" fill={ch >= 0 ? OK : RED}>
                {ch >= 0 ? "▲" : "▼"} {Math.abs(ch).toFixed(1)}%
              </text>
            )}
          </g>
        );
      })}
    </svg>
  );
}

function Donut({ rows, onPick }: { rows: SvcRow[]; onPick: (id: string) => void }) {
  const total = rows.reduce((a, r) => a + r.net, 0);
  if (!total) return <p className="page-sub" style={{ padding: "1rem 0" }}>Məlumat yoxdur</p>;
  const top = rows.slice(0, 5);
  const rest = rows.slice(5).reduce((a, r) => a + r.net, 0);
  const parts = [...top.map((r) => ({ id: r.service_id, name: r.service, v: r.net })),
    ...(rest > 0 ? [{ id: "", name: "Digər", v: rest }] : [])];
  const R = 80, r0 = 46, cx = 110, cy = 100;
  let a0 = -Math.PI / 2;
  const arcs = parts.map((p, i) => {
    const frac = p.v / total;
    const a1 = a0 + frac * Math.PI * 2;
    const large = a1 - a0 > Math.PI ? 1 : 0;
    const sx = cx + R * Math.cos(a0), sy = cy + R * Math.sin(a0);
    const ex = cx + R * Math.cos(a1), ey = cy + R * Math.sin(a1);
    const sx2 = cx + r0 * Math.cos(a1), sy2 = cy + r0 * Math.sin(a1);
    const ex2 = cx + r0 * Math.cos(a0), ey2 = cy + r0 * Math.sin(a0);
    const d = `M ${sx} ${sy} A ${R} ${R} 0 ${large} 1 ${ex} ${ey} L ${sx2} ${sy2} A ${r0} ${r0} 0 ${large} 0 ${ex2} ${ey2} Z`;
    a0 = a1;
    return { d, color: DONUT[i % DONUT.length], ...p, frac };
  });
  return (
    <div style={{ display: "flex", gap: "1.4rem", alignItems: "center", flexWrap: "wrap" }}>
      <svg width={220} height={200} viewBox="0 0 220 200">
        {arcs.map((a) => (
          <path key={a.name} d={a.d} fill={a.color} style={{ cursor: a.id ? "pointer" : "default" }}
            onClick={() => a.id && onPick(a.id)}>
            <title>{a.name}: {azn(a.v)} ({(a.frac * 100).toFixed(1)}%)</title>
          </path>
        ))}
      </svg>
      <div style={{ fontSize: ".85rem", display: "grid", gap: ".35rem" }}>
        {arcs.map((a) => (
          <span key={a.name} style={{ display: "flex", alignItems: "center", gap: ".45rem", cursor: a.id ? "pointer" : "default" }}
            onClick={() => a.id && onPick(a.id)}>
            <span style={{ width: 10, height: 10, background: a.color, display: "inline-block" }} />
            {a.name} — <b>{(a.frac * 100).toFixed(1)}%</b>
          </span>
        ))}
      </div>
    </div>
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

export default function Revenue1C() {
  const conns = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await api.get<{ connections: Conn[] }>("/connections")).data.connections,
  });
  const c1conns = useMemo(
    () => (conns.data ?? []).filter((c) => c.connector_type === "mssql" && (c.source === "1c" || true)),
    [conns.data]);
  const [connID, setConnID] = useState("");
  const [sp] = useSearchParams();
  const urlConn = sp.get("connection") ?? "";
  const conn = connID || urlConn || c1conns[0]?.id || "";

  const [kind, setKind] = useState("12m");
  const [[from, to], setRange] = useState<[string, string]>(preset("12m"));
  const [service, setService] = useState("");
  const [manager, setManager] = useState("");

  const { data: d, isLoading, error } = useQuery({
    queryKey: ["c1rev", conn, from, to, service, manager],
    queryFn: async () => (await api.get<Rev>("/reports/c1/revenue", {
      params: { connection: conn, from, to, service, manager },
    })).data,
    enabled: !!conn, staleTime: 5 * 60 * 1000, retry: false,
  });

  const apply = (kk: string) => { setKind(kk); setRange(preset(kk)); };
  const pickYear = (y: number) => { setKind("custom"); setRange([`${y}-01-01`, `${y + 1}-01-01`]); };
  const err = (error as any)?.response?.data?.error ?? (error as any)?.message;
  const k = d?.kpi;
  const grow = (v?: number) => v === undefined ? undefined : `${v >= 0 ? "▲" : "▼"} ${pct(Math.abs(v))}`;
  const growColor = (v?: number) => v === undefined ? undefined : v >= 0 ? OK : RED;

  if (!conns.isLoading && !c1conns.length) {
    return (<><h1>Gəlir Hesabatı</h1>
      <div className="empty-state"><h2>1C bağlantısı yoxdur</h2><p>İnteqrasiyalar bölməsindən 1C bazanızı qoşun.</p></div></>);
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Gəlir Hesabatı</h1>
          <p className="page-sub">Gəlirin mənbəyi, dinamikası və xidmət üzrə bölgüsü — 1C-dən canlı</p>
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
            {[["month", "Cari ay"], ["quarter", "Son 3 ay"], ["12m", "Son 12 ay"], ["year", "Bu il"]].map(([kk, l]) => (
              <button key={kk} className={"chip" + (kind === kk ? " on" : "")} onClick={() => apply(kk)}>{l}</button>
            ))}
          </div>
          <div style={{ display: "flex", gap: ".5rem", alignItems: "center", fontSize: ".85rem", flexWrap: "wrap" }}>
            <input type="date" value={from} onChange={(e) => { setKind("custom"); setRange([e.target.value, to]); }} style={{ padding: ".4rem .6rem", border: "1px solid #dfe2ea" }} />
            <span>—</span>
            <input type="date" value={to} onChange={(e) => { setKind("custom"); setRange([from, e.target.value]); }} style={{ padding: ".4rem .6rem", border: "1px solid #dfe2ea" }} />
            <select value={service} onChange={(e) => setService(e.target.value)} style={{ padding: ".4rem .6rem", border: "1px solid #dfe2ea", maxWidth: 240 }}>
              <option value="">Bütün xidmətlər</option>
              {d?.options.services.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
            <select value={manager} onChange={(e) => setManager(e.target.value)} style={{ padding: ".4rem .6rem", border: "1px solid #dfe2ea", maxWidth: 200 }}>
              <option value="">Bütün menecerlər</option>
              {d?.options.managers.map((m) => <option key={m.id} value={m.id}>{m.name}</option>)}
            </select>
          </div>
        </div>
      </div>

      {err && <p className="error" style={{ marginTop: "1rem" }}>{err}</p>}
      {isLoading && <p style={{ marginTop: "1rem" }}>1C-dən hesablanır…</p>}

      {d && k && (
        <>
          <div className="cards" style={{ marginTop: "1.2rem" }}>
            <Kpi label="Ümumi gəlir (ƏDV-siz)" value={azn(k.net)}
              sub={k.period_growth_pct !== undefined ? `${grow(k.period_growth_pct)} əvvəlki dövrə` : undefined}
              subColor={growColor(k.period_growth_pct)} />
            <Kpi label="MoM artım" value={k.mom_pct !== undefined ? pct(k.mom_pct) : "—"}
              sub="son ay / əvvəlki ay" subColor={growColor(k.mom_pct)} />
            <Kpi label="YoY artım" value={k.yoy_pct !== undefined ? pct(k.yoy_pct) : "—"}
              sub={`ötən il eyni dövr: ${azn(k.yoy_net)}`} subColor={growColor(k.yoy_pct)} />
            <Kpi label="Orta gəlir / müştəri" value={k.avg_per_customer !== undefined ? azn(k.avg_per_customer) : "—"}
              sub={`${k.active_customers} aktiv müştəri`} />
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.activity} İllər üzrə ümumi gəlir</h2>
              <span className="page-sub" style={{ marginTop: 0 }}>sütuna klik → həmin il seçilir</span></div>
            <div style={{ padding: "12px 16px" }}><YearBars rows={d.years} onPick={pickYear} /></div>
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.db} Xidmətlərin gəlirdə payı</h2>
              <span className="page-sub" style={{ marginTop: 0 }}>{d.period.from} — {d.period.to}</span></div>
            <div style={{ padding: "12px 16px" }}><Donut rows={d.services} onPick={setService} /></div>
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.db} Xidmətlər üzrə detallı cədvəl</h2></div>
            <DataTable rows={d.services} exportName="gelir-hesabati-xidmetler"
              searchHint="Axtar: xidmət, kateqoriya…" rowTitle="Xidmət filtri kimi seç"
              onRowClick={(r) => setService(r.service_id)}
              cols={[
                { key: "category", label: "Kateqoriya", val: (r) => r.category },
                { key: "service", label: "Xidmət", val: (r) => r.service },
                { key: "customers", label: "Müştəri sayı", val: (r) => r.customers, align: "right" },
                { key: "net", label: "Gəlir (ƏDV-siz)", val: (r) => r.net, render: (r) => azn(r.net), align: "right" },
                { key: "share", label: "Gəlirdə pay", val: (r) => r.share_pct, render: (r) => pct(r.share_pct), align: "right" },
                { key: "prev", label: "Əvvəlki dövr", val: (r) => r.prev_net, render: (r) => azn(r.prev_net), align: "right" },
                { key: "chg", label: "Dəyişiklik", val: (r) => r.change_pct ?? 0, align: "right",
                  render: (r) => <span style={{ color: r.change_pct == null ? "#8b90a3" : r.change_pct >= 0 ? OK : RED, fontWeight: 600 }}>
                    {r.change_pct == null ? "—" : grow(r.change_pct)}</span> },
                { key: "avg", label: "Orta gəlir/müştəri", val: (r) => r.avg_per_customer, render: (r) => azn(r.avg_per_customer), align: "right" },
              ] satisfies Col<SvcRow>[]} />
          </div>
        </>
      )}
    </>
  );
}
