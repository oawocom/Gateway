import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";
import DataTable from "../../components/DataTable";
import type { Col } from "../../components/DataTable";

interface Conn { id: string; name: string; connector_type: string; source?: string }
interface Bkt { key: string; label: string; amount: number; count: number }
interface InvRow {
  id: string; number: string; date: string; due: string;
  customer_id: string; customer: string; contract_id: string; contract: string;
  manager: string; amount: number; paid: number; outstanding: number;
  days_overdue: number; bucket: string;
}
interface Recv {
  as_of: string;
  kpi: { outstanding: number; overdue: number; open_invoices: number;
         overdue_ratio_pct?: number; collection_rate_pct?: number };
  aging: Bkt[];
  invoices: InvRow[];
}

const azn = (v: number) => new Intl.NumberFormat("az", { maximumFractionDigits: 0 }).format(v) + " ₼";
const pct = (v: number) => v.toFixed(1).replace(".", ",") + " %";
const dt = (s: string) => new Date(s).toLocaleDateString("az");
const RED = "#c62828", OK = "#16a34a";
const BKT_COLOR: Record<string, string> = {
  current: "#8a8fa3", "1_30": "#e2cf7a", "31_60": "#f0c000", "61_90": "#d98a00", "90_plus": RED,
};

function Kpi({ label, value, sub, subColor }: { label: string; value: string; sub?: string; subColor?: string }) {
  return (
    <div className="card">
      <span>{label}</span>
      <strong style={{ fontSize: "1.3rem" }}>{value}</strong>
      {sub && <small style={{ color: subColor ?? "#8b90a3", fontWeight: 600 }}>{sub}</small>}
    </div>
  );
}

function Aging({ rows, active, onPick }: { rows: Bkt[]; active: string; onPick: (k: string) => void }) {
  const max = Math.max(...rows.map((b) => b.amount), 1);
  return (
    <div style={{ display: "grid", gap: ".55rem", padding: "4px 0" }}>
      {rows.map((b) => (
        <div key={b.key} onClick={() => onPick(active === b.key ? "" : b.key)}
          style={{ display: "grid", gridTemplateColumns: "130px 1fr 170px", gap: ".8rem", alignItems: "center",
                   cursor: "pointer", opacity: active && active !== b.key ? 0.45 : 1 }}>
          <span style={{ fontSize: ".85rem", fontWeight: active === b.key ? 700 : 500 }}>{b.label}</span>
          <div style={{ background: "#eef0f5", height: 18 }}>
            <div style={{ width: `${(b.amount / max) * 100}%`, height: "100%", background: BKT_COLOR[b.key] }} />
          </div>
          <span style={{ fontSize: ".85rem", textAlign: "right" }}><b>{azn(b.amount)}</b> · {b.count} invoice</span>
        </div>
      ))}
    </div>
  );
}

export default function Receivables1C() {
  const conns = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await api.get<{ connections: Conn[] }>("/connections")).data.connections,
  });
  const c1conns = useMemo(
    () => (conns.data ?? []).filter((c) => c.connector_type === "mssql"),
    [conns.data]);
  const [connID, setConnID] = useState("");
  const [sp] = useSearchParams();
  const urlConn = sp.get("connection") ?? "";
  const conn = connID || urlConn || c1conns[0]?.id || "";

  const [asOf, setAsOf] = useState("");
  const [customer, setCustomer] = useState<{ id: string; name: string } | null>(null);
  const [bucket, setBucket] = useState("");

  const { data: d, isLoading, error } = useQuery({
    queryKey: ["c1recv", conn, asOf, customer?.id],
    queryFn: async () => (await api.get<Recv>("/reports/c1/receivables", {
      params: { connection: conn, as_of: asOf || undefined, customer: customer?.id || undefined },
    })).data,
    enabled: !!conn, staleTime: 5 * 60 * 1000, retry: false,
  });

  const err = (error as any)?.response?.data?.error ?? (error as any)?.message;
  const k = d?.kpi;

  const shown = useMemo(() => {
    let rows = d?.invoices ?? [];
    if (bucket) rows = rows.filter((r) => r.bucket === bucket);
    return rows;
  }, [d, bucket]);

  if (!conns.isLoading && !c1conns.length) {
    return (<><h1>Debitor Borcları</h1>
      <div className="empty-state"><h2>1C bağlantısı yoxdur</h2><p>İnteqrasiyalar bölməsindən 1C bazanızı qoşun.</p></div></>);
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Debitor Borcları</h1>
          <p className="page-sub">Ödənilməmiş invoice-lar, yaşlanma və gecikmə — 1C-dən canlı (FIFO bağlama)</p>
        </div>
        <div style={{ display: "flex", gap: ".6rem", alignItems: "center" }}>
          {c1conns.length > 1 && (
            <select value={conn} onChange={(e) => setConnID(e.target.value)} style={{ padding: ".5rem .7rem", border: "1px solid #dfe2ea" }}>
              {c1conns.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
            </select>
          )}
          <label style={{ fontSize: ".85rem", color: "#8b90a3" }}>Tarixə görə:</label>
          <input type="date" value={asOf} onChange={(e) => setAsOf(e.target.value)}
            style={{ padding: ".45rem .6rem", border: "1px solid #dfe2ea" }} />
        </div>
      </div>

      {(customer || bucket) && (
        <p className="info-bar" style={{ marginTop: ".8rem" }}>
          Filtr:
          {customer && <> müştəri <b>{customer.name}</b> <button className="ibtn" onClick={() => setCustomer(null)}>✕</button></>}
          {bucket && <> yaş qrupu <b>{d?.aging.find((b) => b.key === bucket)?.label}</b> <button className="ibtn" onClick={() => setBucket("")}>✕</button></>}
        </p>
      )}
      {err && <p className="error" style={{ marginTop: "1rem" }}>{err}</p>}
      {isLoading && <p style={{ marginTop: "1rem" }}>1C-dən hesablanır…</p>}

      {d && k && (
        <>
          <div className="cards" style={{ marginTop: "1.2rem" }}>
            <Kpi label="Ümumi ödənilməmiş" value={azn(k.outstanding)} sub={`${k.open_invoices} açıq invoice · ${d.as_of} tarixinə`} />
            <Kpi label="Gecikmiş məbləğ" value={azn(k.overdue)} subColor={RED}
              sub={k.overdue_ratio_pct !== undefined ? `ümuminin ${pct(k.overdue_ratio_pct)}-i` : undefined} />
            <Kpi label="Overdue Ratio" value={k.overdue_ratio_pct !== undefined ? pct(k.overdue_ratio_pct) : "—"}
              sub="gecikmiş / ödənilməmiş" subColor={k.overdue_ratio_pct !== undefined && k.overdue_ratio_pct > 50 ? RED : OK} />
            <Kpi label="Yığım faizi (12 ay)" value={k.collection_rate_pct !== undefined ? pct(k.collection_rate_pct) : "—"}
              sub="ödənilmiş / hesablanmış" subColor={k.collection_rate_pct !== undefined && k.collection_rate_pct >= 90 ? OK : undefined} />
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.activity} Yaşlanma (aging)</h2>
              <span className="page-sub" style={{ marginTop: 0 }}>zolağa klik → cədvəl filtrlənir</span></div>
            <div style={{ padding: "8px 16px 14px" }}>
              <Aging rows={d.aging} active={bucket} onPick={setBucket} />
            </div>
          </div>

          <div className="panel">
            <div className="panel-head">
              <h2>{Ic.db} İnvoice səviyyəsində detallar</h2>
            </div>
            <DataTable rows={shown} exportName="debitor-borclari"
              searchHint="Axtar: müştəri, №, müqavilə…"
              searchVal={(r) => `${r.customer} ${r.number} ${r.contract}`}
              rowTitle="Müştəriyə görə filtrlə"
              onRowClick={(r) => setCustomer({ id: r.customer_id, name: r.customer })}
              cols={[
                { key: "customer", label: "Müştəri", val: (r) => r.customer,
                  render: (r) => <b>{r.customer}</b> },
                { key: "contract", label: "Müqavilə", val: (r) => r.contract },
                { key: "number", label: "Invoice №", val: (r) => r.number },
                { key: "date", label: "Tarix", val: (r) => r.date.slice(0, 10), render: (r) => dt(r.date) },
                { key: "due", label: "Due date", val: (r) => r.due.slice(0, 10), render: (r) => dt(r.due) },
                { key: "amount", label: "Məbləğ", val: (r) => r.amount, render: (r) => azn(r.amount), align: "right" },
                { key: "paid", label: "Ödənilmiş", val: (r) => r.paid, render: (r) => azn(r.paid), align: "right" },
                { key: "out", label: "Qalıq", val: (r) => r.outstanding, align: "right",
                  render: (r) => <b>{azn(r.outstanding)}</b> },
                { key: "days", label: "Gecikmə (gün)", val: (r) => r.days_overdue, align: "right",
                  render: (r) => <span style={{ color: r.days_overdue > 0 ? RED : OK, fontWeight: 600 }}>
                    {r.days_overdue > 0 ? r.days_overdue : "vaxtında"}</span> },
              ] satisfies Col<InvRow>[]} />
          </div>
        </>
      )}
    </>
  );
}
