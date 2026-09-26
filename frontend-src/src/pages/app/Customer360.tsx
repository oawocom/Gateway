import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";
import DataTable from "../../components/DataTable";
import type { Col } from "../../components/DataTable";

interface Conn { id: string; name: string; connector_type: string; source?: string }
interface Hit { id: string; name: string; voen: string }
interface MonthRow { year: number; month: number; invoiced: number; net: number; paid: number }
interface SvcRow { service_id: string; service: string; category: string; net: number; customers: number; lines: number; active: boolean; monthly_avg: number }
interface Contract { id: string; code: string; name: string; due_days: number; due_days_set: boolean }
interface InvRow {
  id: string; number: string; date: string; due: string; contract: string; manager: string;
  amount: number; paid: number; outstanding: number; status: "paid" | "partial" | "open" | "overdue";
}
interface C360 {
  header: { id: string; name: string; voen: string; status: string; first_invoice: string | null; last_invoice: string | null; manager: string; data_until: string };
  kpi: { last_month_net: number; monthly_avg_net: number; total_net: number; outstanding: number; overdue: number; active_services: number };
  monthly: MonthRow[]; services: SvcRow[]; contracts: Contract[]; invoices: InvRow[];
}

const azn = (v: number) => new Intl.NumberFormat("az", { maximumFractionDigits: 0 }).format(v) + " ₼";
const dt = (s: string) => new Date(s).toLocaleDateString("az");
const MON = ["Yan", "Fev", "Mar", "Apr", "May", "İyn", "İyl", "Avq", "Sen", "Okt", "Noy", "Dek"];
const GOLD = "#f0c000", GRAY = "#8a8fa3", RED = "#c62828", OK = "#16a34a";
const STATUS: Record<InvRow["status"], [string, string]> = {
  paid: ["Ödənilib", OK], partial: ["Qismən", "#d98a00"], open: ["Açıq", GRAY], overdue: ["Gecikmiş", RED],
};

function PayLine({ rows }: { rows: MonthRow[] }) {
  if (!rows.length) return <p className="page-sub" style={{ padding: "1rem 0" }}>Məlumat yoxdur</p>;
  const w = 760, h = 220, pad = 46;
  const max = Math.max(...rows.map((r) => Math.max(r.invoiced, r.paid)), 1);
  const x = (i: number) => pad + (i * (w - pad * 2)) / Math.max(rows.length - 1, 1);
  const y = (v: number) => h - pad - ((h - pad * 2) * v) / max;
  const path = (get: (r: MonthRow) => number) => rows.map((r, i) => `${i ? "L" : "M"} ${x(i)} ${y(get(r))}`).join(" ");
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: w }}>
      <line x1={pad} y1={h - pad} x2={w - pad} y2={h - pad} stroke="#e4e6ee" />
      <text x={pad - 6} y={pad + 4} textAnchor="end" fontSize="10" fill="#888">{azn(max)}</text>
      <path d={path((r) => r.invoiced)} fill="none" stroke={GRAY} strokeWidth={2} strokeDasharray="5 4" />
      <path d={path((r) => r.paid)} fill="none" stroke={GOLD} strokeWidth={2.5} />
      {rows.map((r, i) => (
        <g key={i}>
          <title>{MON[r.month - 1]} {r.year}: hesablanmış {azn(r.invoiced)}, ödənilmiş {azn(r.paid)}</title>
          <circle cx={x(i)} cy={y(r.paid)} r={3.2} fill={GOLD} />
          <circle cx={x(i)} cy={y(r.invoiced)} r={2.6} fill={GRAY} />
          <text x={x(i)} y={h - pad + 14} textAnchor="middle" fontSize="9.5" fill="#555">{MON[r.month - 1]}</text>
          {(i === 0 || r.month === 1) && <text x={x(i)} y={h - pad + 26} textAnchor="middle" fontSize="9" fill="#999">{r.year}</text>}
        </g>
      ))}
      <g>
        <line x1={pad} y1={12} x2={pad + 24} y2={12} stroke={GRAY} strokeWidth={2} strokeDasharray="5 4" />
        <text x={pad + 30} y={16} fontSize="10">Hesablanmış</text>
        <line x1={pad + 120} y1={12} x2={pad + 144} y2={12} stroke={GOLD} strokeWidth={2.5} />
        <text x={pad + 150} y={16} fontSize="10">Ödənilmiş</text>
      </g>
    </svg>
  );
}

function Kpi({ label, value, sub, subColor }: { label: string; value: string; sub?: string; subColor?: string }) {
  return (
    <div className="card">
      <span>{label}</span>
      <strong style={{ fontSize: "1.25rem" }}>{value}</strong>
      {sub && <small style={{ color: subColor ?? "#8b90a3", fontWeight: 600 }}>{sub}</small>}
    </div>
  );
}

const TABS = ["Overview", "Financial", "Müqavilələr", "Xidmətlər", "İnvoicelar"] as const;

export default function Customer360() {
  const [params, setParams] = useSearchParams();
  const custID = params.get("customer") ?? "";

  const conns = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await api.get<{ connections: Conn[] }>("/connections")).data.connections,
  });
  const c1conns = useMemo(() => (conns.data ?? []).filter((c) => c.connector_type === "mssql"), [conns.data]);
  const conn = params.get("connection") || c1conns[0]?.id || "";

  const [search, setSearch] = useState("");
  const [tab, setTab] = useState<(typeof TABS)[number]>("Overview");

  const hits = useQuery({
    queryKey: ["c1csearch", conn, search],
    queryFn: async () => (await api.get<{ customers: Hit[] }>("/reports/c1/customersearch", {
      params: { connection: conn, q: search },
    })).data.customers,
    enabled: !!conn && !custID, staleTime: 60 * 1000, retry: false,
  });

  const { data: d, isLoading, error } = useQuery({
    queryKey: ["c1c360", conn, custID],
    queryFn: async () => (await api.get<C360>("/reports/c1/customer360", {
      params: { connection: conn, customer: custID },
    })).data,
    enabled: !!conn && !!custID, staleTime: 5 * 60 * 1000, retry: false,
  });

  const err = (error as any)?.response?.data?.error ?? (error as any)?.message;

  if (!conns.isLoading && !c1conns.length) {
    return (<><h1>Customer 360°</h1>
      <div className="empty-state"><h2>1C bağlantısı yoxdur</h2><p>İnteqrasiyalar bölməsindən 1C bazanızı qoşun.</p></div></>);
  }

  if (!custID) {
    return (
      <>
        <h1>Customer 360°</h1>
        <p className="page-sub">Müştərini seçin — maliyyə, müqavilə və xidmət mənzərəsi bir səhifədə</p>
        <div className="panel" style={{ marginTop: "1rem", maxWidth: 560 }}>
          <div style={{ padding: "1rem 1.2rem" }}>
            <input autoFocus placeholder="Ad və ya VÖEN ilə axtar…" value={search}
              onChange={(e) => setSearch(e.target.value)}
              style={{ width: "100%", padding: ".6rem .8rem", border: "1px solid #dfe2ea" }} />
            <div style={{ marginTop: ".6rem", display: "grid", gap: ".3rem" }}>
              {hits.data?.map((h) => (
                <div key={h.id} onClick={() => setParams(conn ? { customer: h.id, connection: conn } : { customer: h.id })}
                  style={{ padding: ".55rem .7rem", border: "1px solid #eef0f5", cursor: "pointer", display: "flex", justifyContent: "space-between" }}>
                  <b>{h.name}</b><span style={{ color: "#8b90a3" }}>{h.voen || "—"}</span>
                </div>
              ))}
              {hits.isLoading && <p className="page-sub">Axtarılır…</p>}
            </div>
          </div>
        </div>
      </>
    );
  }

  return (
    <>
      {err && <p className="error" style={{ marginTop: "1rem" }}>{err}</p>}
      {isLoading && <p style={{ marginTop: "1rem" }}>1C-dən yığılır…</p>}
      {d && (
        <>
          <div className="page-head">
            <div>
              <p className="page-sub" style={{ marginBottom: ".2rem" }}>
                <a style={{ color: "#8b90a3", cursor: "pointer" }} onClick={() => setParams(conn ? { connection: conn } : {})}>Customer 360°</a> / {d.header.name}
              </p>
              <h1 style={{ display: "flex", alignItems: "center", gap: ".6rem" }}>
                {d.header.name}
                <span style={{
                  fontSize: ".72rem", padding: ".2rem .55rem", fontWeight: 700,
                  background: d.header.status === "Aktiv" ? "#e8f5ec" : "#fdecea",
                  color: d.header.status === "Aktiv" ? OK : RED,
                }}>{d.header.status}</span>
              </h1>
              <p className="page-sub" style={{ marginTop: ".25rem" }}>
                VÖEN: {d.header.voen || "—"} · İlk aktivləşmə: {d.header.first_invoice ? dt(d.header.first_invoice) : "—"} ·
                Son invoice: {d.header.last_invoice ? dt(d.header.last_invoice) : "—"} ·
                Məsul menecer: {d.header.manager || "1C-də qeyd olunmayıb"}
              </p>
            </div>
          </div>

          <div className="cards" style={{ marginTop: ".6rem" }}>
            <Kpi label="Son ay gəliri" value={azn(d.kpi.last_month_net)} sub={`orta aylıq: ${azn(d.kpi.monthly_avg_net)}`} />
            <Kpi label="Ümumi gəlir (bütün dövr)" value={azn(d.kpi.total_net)} />
            <Kpi label="Ödənilməmiş" value={azn(d.kpi.outstanding)}
              sub={d.kpi.overdue > 0 ? `gecikmiş: ${azn(d.kpi.overdue)}` : "gecikmə yoxdur"}
              subColor={d.kpi.overdue > 0 ? RED : OK} />
            <Kpi label="Aktiv xidmət sayı" value={String(d.kpi.active_services)} sub="son 3 ayda faktura olunan" />
          </div>

          <div className="chips" style={{ marginTop: "1rem" }}>
            {TABS.map((t) => (
              <button key={t} className={"chip" + (tab === t ? " on" : "")} onClick={() => setTab(t)}>{t}</button>
            ))}
          </div>

          {tab === "Overview" && (
            <div className="panel">
              <div className="panel-head"><h2>{Ic.activity} Ödəniş tarixçəsi — son 12 ay</h2></div>
              <div style={{ padding: "12px 16px" }}><PayLine rows={d.monthly} /></div>
            </div>
          )}

          {tab === "Financial" && (
            <div className="panel">
              <div className="panel-head"><h2>{Ic.db} Açıq invoice-lar</h2></div>
              <DataTable rows={d.invoices.filter((v) => v.status !== "paid")} exportName="acik-invoicelar"
                cols={[
                  { key: "no", label: "Invoice №", val: (v) => v.number },
                  { key: "date", label: "Tarix", val: (v) => v.date.slice(0, 10), render: (v) => dt(v.date) },
                  { key: "due", label: "Due date", val: (v) => v.due.slice(0, 10), render: (v) => dt(v.due) },
                  { key: "amount", label: "Məbləğ", val: (v) => v.amount, render: (v) => azn(v.amount), align: "right" },
                  { key: "paid", label: "Ödənilmiş", val: (v) => v.paid, render: (v) => azn(v.paid), align: "right" },
                  { key: "out", label: "Qalıq", val: (v) => v.outstanding, align: "right", render: (v) => <b>{azn(v.outstanding)}</b> },
                  { key: "st", label: "Status", val: (v) => STATUS[v.status][0],
                    render: (v) => <span style={{ color: STATUS[v.status][1], fontWeight: 600 }}>{STATUS[v.status][0]}</span> },
                ] satisfies Col<InvRow>[]} />
            </div>
          )}

          {tab === "Müqavilələr" && (
            <div className="panel">
              <div className="panel-head"><h2>{Ic.db} Müqavilələr</h2></div>
              <DataTable rows={d.contracts} exportName="muqavileler"
                cols={[
                  { key: "code", label: "Kod", val: (c) => c.code },
                  { key: "name", label: "Müqavilə", val: (c) => c.name },
                  { key: "due", label: "Ödəniş şərti", val: (c) => c.due_days_set ? `${c.due_days} gün` : "təyin olunmayıb (standart 14 gün)" },
                ] satisfies Col<Contract>[]} />
            </div>
          )}

          {tab === "Xidmətlər" && (
            <div className="panel">
              <div className="panel-head"><h2>{Ic.db} Xidmətlər — son 12 ay</h2>
                <span className="page-sub" style={{ marginTop: 0 }}>tarif/sürət 1C-də ayrıca saxlanmır — aylıq orta faktura göstərilir</span></div>
              <DataTable rows={d.services} exportName="musteri-xidmetler"
                cols={[
                  { key: "svc", label: "Xidmət", val: (sv) => sv.service, render: (sv) => <b>{sv.service}</b> },
                  { key: "cat", label: "Kateqoriya", val: (sv) => sv.category },
                  { key: "net", label: "12 aylıq gəlir", val: (sv) => sv.net, render: (sv) => azn(sv.net), align: "right" },
                  { key: "avg", label: "Aylıq orta", val: (sv) => sv.monthly_avg, render: (sv) => azn(sv.monthly_avg), align: "right" },
                  { key: "st", label: "Status", val: (sv) => sv.active ? "Aktiv" : "Passiv",
                    render: (sv) => <span style={{ color: sv.active ? OK : GRAY, fontWeight: 600 }}>{sv.active ? "Aktiv" : "Passiv"}</span> },
                ] satisfies Col<SvcRow>[]} />
            </div>
          )}

          {tab === "İnvoicelar" && (
            <div className="panel">
              <div className="panel-head"><h2>{Ic.db} İnvoicelar (son {d.invoices.length})</h2></div>
              <DataTable rows={d.invoices} exportName="musteri-invoicelar"
                searchHint="Axtar: №, müqavilə…" searchVal={(v) => `${v.number} ${v.contract}`}
                cols={[
                  { key: "no", label: "№", val: (v) => v.number },
                  { key: "date", label: "Tarix", val: (v) => v.date.slice(0, 10), render: (v) => dt(v.date) },
                  { key: "contract", label: "Müqavilə", val: (v) => v.contract },
                  { key: "amount", label: "Məbləğ", val: (v) => v.amount, render: (v) => azn(v.amount), align: "right" },
                  { key: "paid", label: "Ödənilmiş", val: (v) => v.paid, render: (v) => azn(v.paid), align: "right" },
                  { key: "out", label: "Qalıq", val: (v) => v.outstanding, align: "right",
                    render: (v) => v.outstanding > 0 ? azn(v.outstanding) : "—" },
                  { key: "st", label: "Status", val: (v) => STATUS[v.status][0],
                    render: (v) => <span style={{ color: STATUS[v.status][1], fontWeight: 600 }}>{STATUS[v.status][0]}</span> },
                ] satisfies Col<InvRow>[]} />
            </div>
          )}
        </>
      )}
    </>
  );
}
