import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";

// ---- API types (mirror backend/internal/api/reportsc1.go) ----
interface Conn { id: string; name: string; connector_type: string; status: string }
interface KPI {
  invoiced: number; revenue_net: number; paid: number; paid_invoiced: number; paid_non_invoiced: number;
  collection_rate: number; outstanding: number; overdue: number; overdue_ratio: number; advances: number;
  non_invoiced_balance: number; invoices: number; active_customers: number; new_customers: number;
  lost_customers: number; open_invoices: number;
  outstanding_start: number; overdue_start: number; outstanding_period: number; overdue_period: number; open_period: number;
}
interface MonthRow { year: number; month: number; invoiced: number; net: number; paid: number; invoices: number }
interface ServiceRow { service_id: string; service: string; category: string; net: number; vat: number; customers: number; lines: number }
interface CustomerRow { customer_id: string; customer: string; voen: string; invoiced: number; paid: number; invoices: number }
interface Bucket { label: string; amount: number; count: number }
interface OverdueCust { customer_id: string; customer: string; overdue: number; invoices: number; max_days: number }
interface Advance { customer_id: string; customer: string; contract_id: string; contract: string; amount: number; non_invoiced: boolean }
interface Dash {
  info: { adapter: string; config_name: string; config_version: string };
  period: { from: string; to: string; as_of: string; data_until: string; due_days_default: number };
  kpi: KPI; monthly: MonthRow[]; services: ServiceRow[]; top_customers: CustomerRow[];
  aging: Bucket[]; top_overdue: OverdueCust[]; advances: Advance[]; other_payers: Advance[];
}

// ---- formatting (doc §11: thousands separator, 1 decimal for %) ----
const azn = (v: number) => new Intl.NumberFormat("az", { maximumFractionDigits: 0 }).format(v) + " ₼";
const pct = (v: number) => v.toFixed(1).replace(".", ",") + " %";
const num = (v: number) => new Intl.NumberFormat("az").format(v);
const MONTHS = ["Yan", "Fev", "Mar", "Apr", "May", "İyn", "İyl", "Avq", "Sen", "Okt", "Noy", "Dek"];
const ml = (m: { year: number; month: number }) => `${MONTHS[m.month - 1]} ${String(m.year).slice(2)}`;
const iso = (d: Date) => d.toISOString().slice(0, 10);
const GOLD = "#f0c000", INK = "#8a8fa3", RED = "#c62828", GRAY = "#e4e6ee", OK = "#16a34a";

// ---- period presets, by the calendar (today) ----
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

// last 12 months of data, for the "jump to data" action when the period is empty
function dataRange(until: string): [string, string] {
  const u = new Date(until);
  const first = new Date(Date.UTC(u.getUTCFullYear(), u.getUTCMonth(), 1));
  const add = (d: Date, m: number) => new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + m, 1));
  return [iso(add(first, -11)), iso(add(first, 1))];
}

const dmy = (s: string) => (s && s.length >= 10 ? `${s.slice(8, 10)}.${s.slice(5, 7)}.${s.slice(0, 4)}` : s);

// previous period of equal length, for KPI comparison
function previous(from: string, to: string): [string, string] {
  const a = new Date(from), b = new Date(to);
  const len = b.getTime() - a.getTime();
  return [iso(new Date(a.getTime() - len)), from];
}

function delta(cur: number, prev: number | undefined) {
  if (prev === undefined || prev === 0) return null;
  return ((cur - prev) / Math.abs(prev)) * 100;
}

// ---- charts (inline SVG, same style as Reports.tsx) ----
function Lines({ rows }: { rows: MonthRow[] }) {
  if (!rows.length) return <Empty />;
  const w = 720, h = 210, pad = 44;
  const max = Math.max(...rows.flatMap((r) => [r.invoiced, r.paid, r.net]), 1);
  const step = rows.length > 1 ? (w - pad * 2) / (rows.length - 1) : 0;
  const x = (i: number) => pad + i * step;
  const y = (v: number) => h - pad + 10 - ((h - pad * 2) * v) / max;
  const line = (f: (r: MonthRow) => number) => rows.map((r, i) => `${x(i)},${y(f(r))}`).join(" ");
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: w }}>
      <line x1={pad} y1={h - pad + 10} x2={w - pad} y2={h - pad + 10} stroke={GRAY} />
      <text x={pad - 6} y={pad - 4} textAnchor="end" fontSize="10" fill="#888">{azn(max)}</text>
      <text x={pad - 6} y={h - pad + 14} textAnchor="end" fontSize="10" fill="#888">0</text>
      {rows.length > 1 && <>
        <polyline points={line((r) => r.invoiced)} fill="none" stroke={GOLD} strokeWidth="2.2" />
        <polyline points={line((r) => r.paid)} fill="none" stroke={OK} strokeWidth="2" />
        <polyline points={line((r) => r.net)} fill="none" stroke={INK} strokeWidth="1.5" strokeDasharray="4 3" />
      </>}
      {rows.map((r, i) => (
        <g key={`${r.year}-${r.month}`}>
          <circle cx={x(i)} cy={y(r.invoiced)} r="3" fill={GOLD}><title>{ml(r)}: hesablanmış {azn(r.invoiced)}</title></circle>
          <circle cx={x(i)} cy={y(r.paid)} r="3" fill={OK}><title>{ml(r)}: ödənilmiş {azn(r.paid)}</title></circle>
          <text x={x(i)} y={h - pad + 26} textAnchor="middle" fontSize="9" fill="#888">{ml(r)}</text>
        </g>
      ))}
    </svg>
  );
}

function HBars({ rows, color, onClick }: { rows: { key: string; label: string; value: number; sub?: string }[]; color?: string; onClick?: (key: string) => void }) {
  if (!rows.length) return <Empty />;
  const max = Math.max(...rows.map((r) => r.value), 1);
  const rowH = 26, w = 720, labelW = 260;
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${rows.length * rowH + 4}`} style={{ maxWidth: w }}>
      {rows.map((r, i) => {
        const bw = Math.max(((w - labelW - 110) * r.value) / max, 2);
        const y = i * rowH;
        return (
          <g key={r.key} style={{ cursor: onClick ? "pointer" : "default" }} onClick={() => onClick?.(r.key)}>
            <title>{r.label}: {azn(r.value)}{r.sub ? ` (${r.sub})` : ""}</title>
            <text x={labelW - 8} y={y + 17} textAnchor="end" fontSize="12" fill="#444">
              {r.label.length > 34 ? r.label.slice(0, 32) + "…" : r.label}
            </text>
            <rect x={labelW} y={y + 5} width={bw} height={16} fill={color ?? GOLD} />
            <text x={labelW + bw + 6} y={y + 17} fontSize="12" fill="#111" fontWeight="600">{azn(r.value)}</text>
          </g>
        );
      })}
    </svg>
  );
}

function Empty() { return <p className="page-sub" style={{ padding: "1rem 0" }}>Məlumat yoxdur</p>; }

function Legend({ items }: { items: [string, string][] }) {
  return (
    <div style={{ display: "flex", gap: 16, padding: "0 16px", fontSize: 12, color: "#555", flexWrap: "wrap" }}>
      {items.map(([c, l]) => (
        <span key={l} style={{ display: "flex", alignItems: "center", gap: 5 }}>
          <span style={{ width: 10, height: 10, background: c, display: "inline-block" }} /> {l}
        </span>
      ))}
    </div>
  );
}

function Kpi({ label, value, prev, help, invert, onClick }: { label: string; value: string; prev?: number | null; help?: string; invert?: boolean; onClick?: () => void }) {
  const good = prev == null ? null : invert ? prev < 0 : prev > 0;
  return (
    <div className="card" title={help} style={{ cursor: onClick ? "pointer" : "default" }} onClick={onClick}>
      <span>{label}</span>
      <strong style={{ fontSize: "1.3rem" }}>{value}</strong>
      {prev != null && (
        <small style={{ color: good ? OK : RED, fontWeight: 600 }}>
          {prev > 0 ? "▲" : "▼"} {pct(Math.abs(prev))} <span style={{ color: "#8b90a3", fontWeight: 400 }}>əvvəlki dövrə</span>
        </small>
      )}
    </div>
  );
}

// stock figure with its opening value: "dövr əvvəli → dövr sonu"
function Balance({ label, start, end, from, help, onClick }: { label: string; start: number; end: number; from: string; help?: string; onClick?: () => void }) {
  const diff = end - start;
  return (
    <div className="card" title={help} style={{ cursor: onClick ? "pointer" : "default" }} onClick={onClick}>
      <span>{label}</span>
      <strong style={{ fontSize: "1.3rem" }}>{azn(end)}</strong>
      <small style={{ color: "#8b90a3" }}>
        {from}-ə: {azn(start)} · dəyişim{" "}
        <b style={{ color: diff > 0 ? RED : diff < 0 ? OK : "#8b90a3" }}>{diff > 0 ? "▲" : diff < 0 ? "▼" : "="} {azn(Math.abs(diff))}</b>
      </small>
    </div>
  );
}

export default function Dashboard1C() {
  const conns = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await api.get<{ connections: Conn[] }>("/connections")).data.connections,
  });
  const mssql = useMemo(() => (conns.data ?? []).filter((c) => c.connector_type === "mssql"), [conns.data]);
  const [connID, setConnID] = useState("");
  const [sp] = useSearchParams();
  const urlConn = sp.get("connection") ?? "";
  const conn = connID || urlConn || mssql[0]?.id || "";

  const [kind, setKind] = useState("12m");
  const [[from, to], setRange] = useState<[string, string]>(["", ""]); // empty = server default (last 12 months of data)
  const [customer, setCustomer] = useState<{ id: string; name: string } | null>(null);
  const [detail, setDetail] = useState<"overdue" | "services" | "customers" | "other" | null>(null);

  const params = (f: string, t: string) => ({ connection: conn, from: f, to: t, customer: customer?.id ?? "" });
  const cur = useQuery({
    queryKey: ["c1dash", conn, from, to, customer?.id],
    queryFn: async () => (await api.get<Dash>("/reports/c1/dashboard", { params: params(from, to) })).data,
    enabled: !!conn, staleTime: 5 * 60 * 1000, retry: false,
  });
  const ef = from || cur.data?.period.from || "", et = to || cur.data?.period.to || "";
  const [pf, pt] = ef && et ? previous(ef, et) : ["", ""];
  const prev = useQuery({
    queryKey: ["c1dash", conn, pf, pt, customer?.id],
    queryFn: async () => (await api.get<Dash>("/reports/c1/dashboard", { params: params(pf, pt) })).data,
    enabled: !!conn && !!pf, staleTime: 5 * 60 * 1000, retry: false,
  });

  if (conns.isLoading) return <p>Yüklənir...</p>;
  if (!mssql.length) {
    return (
      <>
        <h1>Rəhbər paneli</h1>
        <div className="empty-state"><h2>1C bağlantısı yoxdur</h2><p>Bağlantılarım bölməsində 1C-nin MSSQL bazasına bağlantı əlavə edin.</p></div>
      </>
    );
  }
  const d = cur.data, k = d?.kpi, pk = prev.data?.kpi;
  const err = (cur.error as any)?.response?.data?.error ?? (cur.error as any)?.message;

  const apply = (kk: string) => { setKind(kk); setRange(preset(kk)); };
  const jumpToData = () => { if (d?.period.data_until) { setKind("custom"); setRange(dataRange(d.period.data_until)); } };
  const emptyPeriod = !!d && d.monthly.length === 0;
  const setFrom = (v: string) => { setKind("custom"); setRange([v, et]); };
  const setTo = (v: string) => { setKind("custom"); setRange([ef, v]); };

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Rəhbər paneli</h1>
          <p className="page-sub">
            1C mühasibatından canlı — {d ? `${d.info.config_name} ${d.info.config_version}` : "…"}
            {d && ` · bazada son məlumat ${dmy(d.period.data_until)} · ödəniş müddəti default ${d.period.due_days_default} gün`}
          </p>
        </div>
        {mssql.length > 1 && (
          <select value={conn} onChange={(e) => setConnID(e.target.value)} style={{ padding: ".5rem .7rem", border: "1px solid #dfe2ea" }}>
            {mssql.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
        )}
      </div>

      {/* 1. filter panel */}
      <div className="panel" style={{ marginTop: "1rem" }}>
        <div className="panel-head" style={{ flexWrap: "wrap", gap: ".6rem" }}>
          <div className="chips" style={{ marginTop: 0 }}>
            {[["month", "Cari ay"], ["quarter", "Son 3 ay"], ["12m", "Son 12 ay"], ["year", "Bu il"]].map(([kk, l]) => (
              <button key={kk} className={"chip" + (kind === kk ? " on" : "")} onClick={() => apply(kk)}>{l}</button>
            ))}
          </div>
          <div style={{ display: "flex", gap: ".5rem", alignItems: "center", fontSize: ".85rem" }}>
            <input type="date" value={ef} onChange={(e) => setFrom(e.target.value)} style={{ padding: ".4rem .6rem", border: "1px solid #dfe2ea" }} />
            <span>—</span>
            <input type="date" value={et} onChange={(e) => setTo(e.target.value)} style={{ padding: ".4rem .6rem", border: "1px solid #dfe2ea" }} />
            {customer && (
              <button className="chip on" onClick={() => setCustomer(null)} title="Müştəri filtrini sil">
                {customer.name.slice(0, 28)} ✕
              </button>
            )}
            <button className="ibtn" title="Yenilə" onClick={() => { cur.refetch(); prev.refetch(); }}>{Ic.refresh}</button>
          </div>
        </div>
      </div>

      {err && <p className="error" style={{ marginTop: "1rem" }}>{err}</p>}
      {cur.isLoading && <p style={{ marginTop: "1rem" }}>1C-dən hesablanır…</p>}

      {d && emptyPeriod && (
        <div className="panel" style={{ marginTop: "1rem", borderLeft: `4px solid ${RED}` }}>
          <div className="panel-head" style={{ gap: "1rem" }}>
            <div>
              <strong>Seçilən dövr üçün 1C-də sənəd yoxdur.</strong>
              <div className="page-sub">Bazada son məlumat: {dmy(d.period.data_until)}. Aşağıdakı qalıq kartları {dmy(d.period.as_of)} tarixinə hesablanıb.</div>
            </div>
            <button className="btn-primary sm" onClick={jumpToData}>Son məlumat dövrünə keç</button>
          </div>
        </div>
      )}

      {d && k && (
        <>
          {/* 2. KPI cards (doc §3.1 / §13.2) */}
          <div className="cards" style={{ marginTop: "1.2rem" }}>
            <Kpi label="Ümumi gəlir (ƏDV-siz)" value={azn(k.revenue_net)} prev={delta(k.revenue_net, pk?.revenue_net)} help="Invoice sətirlərinin cəmi, ƏDV-siz" />
            <Kpi label="Hesablanmış (ƏDV ilə)" value={azn(k.invoiced)} prev={delta(k.invoiced, pk?.invoiced)} help={`${num(k.invoices)} invoice`} />
            <Kpi label="Ödənilmiş — korporativ" value={azn(k.paid_invoiced)} prev={delta(k.paid_invoiced, pk?.paid_invoiced)} help="Invoice-lu müştərilərdən daxil olan" />
            <Kpi label="Yığım faizi" value={pct(k.collection_rate)} prev={delta(k.collection_rate, pk?.collection_rate)} help="Ödənilmiş (korporativ) / Hesablanmış" />
            <Kpi label="Dövrdə hesablanıb, ödənilməyib" value={azn(k.outstanding_period)} help={`Dövrün invoice-larından qalan · ${num(k.open_period)} invoice`} onClick={() => setDetail("overdue")} />
            <Kpi label="Ondan gecikmiş" value={azn(k.overdue_period)} help="Dövrün invoice-larından ödəniş müddəti keçənlər" onClick={() => setDetail("overdue")} />
            <Kpi label="Aktiv müştərilər" value={num(k.active_customers)} prev={delta(k.active_customers, pk?.active_customers)} help="Dövrdə invoice alan unikal müştərilər" />
            <Kpi label="Yeni / itirilmiş" value={`${num(k.new_customers)} / ${num(k.lost_customers)}`} help="Əvvəlki bərabər dövrə nisbətən" />
          </div>
          <div className="cards" style={{ marginTop: ".8rem" }}>
            <Kpi label="Ödəniş sistemləri / B2C daxilolma" value={azn(k.paid_non_invoiced)} prev={delta(k.paid_non_invoiced, pk?.paid_non_invoiced)} help="1C-də invoice-u olmayan ödəyicilərdən" onClick={() => setDetail("other")} />
            <Kpi label="Ümumi daxilolma" value={azn(k.paid)} prev={delta(k.paid, pk?.paid)} help="Korporativ + ödəniş sistemləri" />
            <Kpi label="Müştəri avansları" value={azn(k.advances)} help="Invoice-dan artıq ödənilmiş məbləğlər" />
          </div>
          <div className="cards" style={{ marginTop: ".8rem" }}>
            <Balance label={`Ümumi qalıq (${dmy(d.period.as_of)}-ə)`} start={k.outstanding_start} end={k.outstanding} from={dmy(d.period.from)} help={`${num(k.open_invoices)} açıq invoice, bütün tarix üzrə`} onClick={() => setDetail("overdue")} />
            <Balance label={`Ondan gecikmiş (${dmy(d.period.as_of)}-ə)`} start={k.overdue_start} end={k.overdue} from={dmy(d.period.from)} help={`Overdue ratio ${pct(k.overdue_ratio)}`} onClick={() => setDetail("overdue")} />
          </div>

          {/* 3. revenue analytics */}
          <div className="panel">
            <div className="panel-head"><h2>{Ic.activity} Aylıq dinamika</h2></div>
            <Legend items={[[GOLD, "Hesablanmış"], [OK, "Ödənilmiş"], [INK, "Gəlir (ƏDV-siz)"]]} />
            <div style={{ padding: "12px 16px" }}><Lines rows={d.monthly} /></div>
          </div>
          <div className="panel">
            <div className="panel-head">
              <h2>{Ic.activity} Xidmətlər üzrə gəlir (ƏDV-siz)</h2>
              <button className="mini-link" onClick={() => setDetail(detail === "services" ? null : "services")}>Cədvəl {Ic.chevR}</button>
            </div>
            <div style={{ padding: "12px 16px" }}>
              <HBars rows={d.services.slice(0, 10).map((s) => ({ key: s.service_id, label: s.category ? `${s.category} › ${s.service}` : s.service, value: s.net, sub: `${s.customers} müştəri` }))} />
            </div>
            {detail === "services" && (
              <table className="tbl" style={{ marginTop: 0, boxShadow: "none" }}>
                <thead><tr><th>Kateqoriya</th><th>Xidmət</th><th>Gəlir (ƏDV-siz)</th><th>ƏDV</th><th>Müştəri</th><th>Sətir</th></tr></thead>
                <tbody>{d.services.map((s) => (
                  <tr key={s.service_id}><td>{s.category}</td><td>{s.service}</td><td>{azn(s.net)}</td><td>{azn(s.vat)}</td><td>{num(s.customers)}</td><td>{num(s.lines)}</td></tr>
                ))}</tbody>
              </table>
            )}
          </div>

          {/* 4. payments & risk */}
          <div className="panel">
            <div className="panel-head"><h2>{Ic.alert} Ödənilməmiş məbləğin müddəti (aging)</h2><span className="page-sub" style={{ marginTop: 0 }}>{d.period.as_of} tarixinə</span></div>
            <div style={{ padding: "12px 16px" }}>
              <HBars color={RED} rows={d.aging.map((b) => ({ key: b.label, label: b.label, value: b.amount, sub: `${b.count} invoice` }))} />
            </div>
          </div>
          <div className="panel">
            <div className="panel-head"><h2>{Ic.alert} TOP gecikmiş borclular</h2></div>
            <div style={{ padding: "12px 16px" }}>
              <HBars color={RED} rows={d.top_overdue.map((c) => ({ key: c.customer_id, label: c.customer, value: c.overdue, sub: `${c.invoices} invoice, max ${c.max_days} gün` }))}
                onClick={(id) => { const c = d.top_overdue.find((x) => x.customer_id === id); if (c) setCustomer({ id: c.customer_id, name: c.customer }); }} />
            </div>
            {detail === "overdue" && (
              <table className="tbl" style={{ marginTop: 0, boxShadow: "none" }}>
                <thead><tr><th>Müştəri</th><th>Gecikmiş</th><th>Invoice</th><th>Maks. gecikmə</th></tr></thead>
                <tbody>{d.top_overdue.map((c) => (
                  <tr key={c.customer_id} style={{ cursor: "pointer" }} onClick={() => setCustomer({ id: c.customer_id, name: c.customer })}>
                    <td>{c.customer}</td><td style={{ color: RED, fontWeight: 600 }}>{azn(c.overdue)}</td><td>{num(c.invoices)}</td><td>{c.max_days} gün</td>
                  </tr>
                ))}</tbody>
              </table>
            )}
          </div>

          {/* 5. customer analytics */}
          <div className="panel">
            <div className="panel-head">
              <h2>{Ic.users} Gəlir üzrə TOP müştərilər</h2>
              <button className="mini-link" onClick={() => setDetail(detail === "customers" ? null : "customers")}>Cədvəl {Ic.chevR}</button>
            </div>
            <div style={{ padding: "12px 16px" }}>
              <HBars rows={d.top_customers.map((c) => ({ key: c.customer_id, label: c.customer, value: c.invoiced, sub: `ödənilmiş ${azn(c.paid)}` }))}
                onClick={(id) => { const c = d.top_customers.find((x) => x.customer_id === id); if (c) setCustomer({ id: c.customer_id, name: c.customer }); }} />
            </div>
            {detail === "customers" && (
              <table className="tbl" style={{ marginTop: 0, boxShadow: "none" }}>
                <thead><tr><th>Müştəri</th><th>VÖEN</th><th>Hesablanmış</th><th>Ödənilmiş</th><th>Invoice</th><th>Payı</th></tr></thead>
                <tbody>{d.top_customers.map((c) => (
                  <tr key={c.customer_id} style={{ cursor: "pointer" }} onClick={() => setCustomer({ id: c.customer_id, name: c.customer })}>
                    <td>{c.customer}</td><td>{c.voen}</td><td>{azn(c.invoiced)}</td><td>{azn(c.paid)}</td><td>{num(c.invoices)}</td>
                    <td>{k.invoiced ? pct((c.invoiced / k.invoiced) * 100) : "—"}</td>
                  </tr>
                ))}</tbody>
              </table>
            )}
          </div>

          {(detail === "other" || d.other_payers.length > 0) && (
            <div className="panel">
              <div className="panel-head">
                <h2>{Ic.db} Ödəniş sistemləri / invoice-suz daxilolma</h2>
                <span className="page-sub" style={{ marginTop: 0 }}>tarixi cəmi {azn(k.non_invoiced_balance)}</span>
              </div>
              <table className="tbl" style={{ marginTop: 0, boxShadow: "none" }}>
                <thead><tr><th>Ödəyici</th><th>Tarixi cəmi</th></tr></thead>
                <tbody>{d.other_payers.slice(0, detail === "other" ? 100 : 6).map((a) => (
                  <tr key={a.contract_id || a.customer_id}><td>{a.customer || "—"}</td><td>{azn(a.amount)}</td></tr>
                ))}</tbody>
              </table>
              {d.other_payers.length > 6 && detail !== "other" && (
                <div style={{ padding: ".4rem 1rem .9rem" }}><button className="mini-link" onClick={() => setDetail("other")}>Hamısı ({d.other_payers.length}) {Ic.chevR}</button></div>
              )}
            </div>
          )}
        </>
      )}
    </>
  );
}
