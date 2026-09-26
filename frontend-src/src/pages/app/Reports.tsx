import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";

interface Entity { connection: string; entity: string; records: number }
interface Day { day: string; records: number; syncs: number; errors: number }
interface Conn { name: string; type: string; status: string; last_sync: string; last_error: string }
interface Summary { entities: Entity[]; daily: Day[]; connections: Conn[] }
interface MV { year: number; month: number; value: number }
interface MP { year: number; month: number; revenue: number; cost: number; profit: number }
interface MC { year: number; month: number; inflow: number; outflow: number }
interface C1 { available: boolean; connection?: string; sales?: MV[]; profit?: MP[]; cash?: MC[]; receivables?: MV[] }

const GOLD = "#f0c000";
const DARK = "#8a8fa3";
const GRAY = "#e4e6ee";
const RED = "#c62828";

function BarChart({ data }: { data: Entity[] }) {
  if (!data.length) return <p className="page-sub">Hələ sinxronlaşmış entity yoxdur</p>;
  const max = Math.max(...data.map((d) => d.records), 1);
  const rowH = 26, w = 640, labelW = 260;
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${data.length * rowH + 4}`} style={{ maxWidth: w }}>
      {data.map((d, i) => {
        const bw = Math.max(((w - labelW - 70) * d.records) / max, 2);
        const y = i * rowH;
        return (
          <g key={d.connection + d.entity}>
            <text x={labelW - 8} y={y + 17} textAnchor="end" fontSize="12" fill="#444">
              {d.entity.length > 32 ? d.entity.slice(0, 30) + "…" : d.entity}
            </text>
            <rect x={labelW} y={y + 5} width={bw} height={16} fill={GOLD} />
            <text x={labelW + bw + 6} y={y + 17} fontSize="12" fill="#111" fontWeight="600">
              {d.records.toLocaleString()}
            </text>
          </g>
        );
      })}
    </svg>
  );
}

function LineChart({ data }: { data: Day[] }) {
  if (!data.length) return <p className="page-sub">Son 14 gündə sinxronizasiya olmayıb</p>;
  const w = 640, h = 180, pad = 36;
  const max = Math.max(...data.map((d) => d.records), 1);
  const step = data.length > 1 ? (w - pad * 2) / (data.length - 1) : 0;
  const x = (i: number) => pad + i * step;
  const y = (v: number) => h - pad - ((h - pad * 2) * v) / max;
  const pts = data.map((d, i) => `${x(i)},${y(d.records)}`).join(" ");
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: w }}>
      <line x1={pad} y1={h - pad} x2={w - pad} y2={h - pad} stroke={GRAY} />
      <line x1={pad} y1={pad} x2={pad} y2={h - pad} stroke={GRAY} />
      <text x={pad - 6} y={pad + 4} textAnchor="end" fontSize="10" fill="#888">{max.toLocaleString()}</text>
      <text x={pad - 6} y={h - pad + 4} textAnchor="end" fontSize="10" fill="#888">0</text>
      {data.length > 1 && <polyline points={pts} fill="none" stroke={GOLD} strokeWidth="2" />}
      {data.map((d, i) => (
        <g key={d.day}>
          <circle cx={x(i)} cy={y(d.records)} r="3.5" fill={d.errors > 0 ? RED : GOLD} />
          <text x={x(i)} y={h - pad + 14} textAnchor="middle" fontSize="9" fill="#888">
            {d.day.slice(5)}
          </text>
        </g>
      ))}
    </svg>
  );
}


const fmt = (v: number) =>
  new Intl.NumberFormat("az", { notation: "compact", maximumFractionDigits: 1 }).format(v);
const ml = (m: { year: number; month: number }) => `${String(m.month).padStart(2, "0")}.${String(m.year).slice(2)}`;

function MonthBars({ rows }: { rows: { label: string; a: number; b?: number }[] }) {
  if (!rows.length) return <p className="page-sub">Data yoxdur</p>;
  const w = 640, h = 200, pad = 30;
  const max = Math.max(...rows.flatMap((r) => [r.a, r.b ?? 0]), 1);
  const gw = (w - pad * 2) / rows.length;
  const bw = rows.some((r) => r.b !== undefined) ? gw * 0.32 : gw * 0.55;
  const y = (v: number) => h - pad - ((h - pad * 2) * v) / max;
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: w }}>
      <line x1={pad} y1={h - pad} x2={w - pad} y2={h - pad} stroke={GRAY} />
      <text x={pad - 4} y={pad + 4} textAnchor="end" fontSize="10" fill="#888">{fmt(max)}</text>
      {rows.map((r, i) => {
        const cx = pad + i * gw + gw / 2;
        return (
          <g key={r.label}>
            <rect x={r.b !== undefined ? cx - bw - 1 : cx - bw / 2} y={y(r.a)} width={bw} height={h - pad - y(r.a)} fill={GOLD} />
            {r.b !== undefined && <rect x={cx + 1} y={y(r.b)} width={bw} height={h - pad - y(r.b)} fill={DARK} />}
            <text x={cx} y={h - pad + 12} textAnchor="middle" fontSize="8.5" fill="#888">{r.label}</text>
          </g>
        );
      })}
    </svg>
  );
}

function MonthLine({ data }: { data: MV[] }) {
  if (!data.length) return <p className="page-sub">Data yoxdur</p>;
  const w = 640, h = 180, pad = 36;
  const max = Math.max(...data.map((d) => d.value), 1);
  const min = Math.min(...data.map((d) => d.value), 0);
  const step = data.length > 1 ? (w - pad * 2) / (data.length - 1) : 0;
  const x = (i: number) => pad + i * step;
  const y = (v: number) => h - pad - ((h - pad * 2) * (v - min)) / (max - min || 1);
  return (
    <svg width="100%" viewBox={`0 0 ${w} ${h}`} style={{ maxWidth: w }}>
      <line x1={pad} y1={h - pad} x2={w - pad} y2={h - pad} stroke={GRAY} />
      <text x={pad - 6} y={pad + 4} textAnchor="end" fontSize="10" fill="#888">{fmt(max)}</text>
      <text x={pad - 6} y={h - pad + 4} textAnchor="end" fontSize="10" fill="#888">{fmt(min)}</text>
      {data.length > 1 && (
        <polyline points={data.map((d, i) => `${x(i)},${y(d.value)}`).join(" ")} fill="none" stroke={GOLD} strokeWidth="2" />
      )}
      {data.map((d, i) => (
        <g key={`${d.year}-${d.month}`}>
          <circle cx={x(i)} cy={y(d.value)} r="3" fill={GOLD} />
          <text x={x(i)} y={h - pad + 13} textAnchor="middle" fontSize="8.5" fill="#888">{ml(d)}</text>
        </g>
      ))}
    </svg>
  );
}

function Legend({ items }: { items: [string, string][] }) {
  return (
    <div style={{ display: "flex", gap: 16, padding: "0 16px", fontSize: 12, color: "#555" }}>
      {items.map(([c, l]) => (
        <span key={l} style={{ display: "flex", alignItems: "center", gap: 5 }}>
          <span style={{ width: 10, height: 10, background: c, display: "inline-block" }} /> {l}
        </span>
      ))}
    </div>
  );
}

export default function Reports() {
  const { data, isLoading } = useQuery({
    queryKey: ["reports-summary"],
    queryFn: async () => (await api.get<Summary>("/reports/summary")).data,
    refetchInterval: 60000,
  });
  const c1 = useQuery({
    queryKey: ["reports-1c"],
    queryFn: async () => (await api.get<C1>("/reports/1c")).data,
    staleTime: 5 * 60 * 1000,
  }).data;

  if (isLoading) return <p>Yüklənir...</p>;

  const totalRecords = data?.entities.reduce((a, e) => a + e.records, 0) ?? 0;
  const totalErrors = data?.daily.reduce((a, d) => a + d.errors, 0) ?? 0;
  const activeConns = data?.connections.filter((c) => c.status === "active").length ?? 0;

  const stats = [
    { icon: Ic.db, label: "Ümumi qeydlər", val: totalRecords.toLocaleString() },
    { icon: Ic.plug, label: "Aktiv bağlantılar", val: `${activeConns} / ${data?.connections.length ?? 0}` },
    { icon: Ic.refresh, label: "Entity sayı", val: data?.entities.length ?? 0 },
    { icon: Ic.alert, label: "Xətalar (14 gün)", val: totalErrors },
  ];

  return (
    <>
      <h1>Hesabatlar</h1>
      <p className="page-sub">İnteqrasiya olunmuş datanın vizual görünüşü</p>

      <div className="cards">
        {stats.map((s) => (
          <div key={s.label} className="card stat">
            <i>{s.icon}</i>
            <div><span>{s.label}</span><strong>{s.val}</strong></div>
          </div>
        ))}
      </div>


      {c1?.available && (
        <>
          <h1 style={{ marginTop: "2rem" }}>1C Mühasibat hesabatları</h1>
          <p className="page-sub">{c1.connection} — canlı data, datanın son 12 ayı</p>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.activity} Aylıq satışlar (Kt 601)</h2></div>
            <div style={{ padding: "12px 16px" }}>
              <MonthBars rows={(c1.sales ?? []).map((m) => ({ label: ml(m), a: m.value }))} />
            </div>
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.activity} Mənfəət dinamikası (601 − 701)</h2></div>
            <Legend items={[[GOLD, "Satış"], [DARK, "Maya dəyəri"]]} />
            <div style={{ padding: "12px 16px" }}>
              <MonthBars rows={(c1.profit ?? []).map((m) => ({ label: ml(m), a: m.revenue, b: m.cost }))} />
            </div>
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.activity} Pul hərəkəti (221, 223)</h2></div>
            <Legend items={[[GOLD, "Mədaxil"], [DARK, "Məxaric"]]} />
            <div style={{ padding: "12px 16px" }}>
              <MonthBars rows={(c1.cash ?? []).map((m) => ({ label: ml(m), a: m.inflow, b: m.outflow }))} />
            </div>
          </div>

          <div className="panel">
            <div className="panel-head"><h2>{Ic.activity} Debitor borc qalığı (211)</h2></div>
            <div style={{ padding: "12px 16px" }}><MonthLine data={c1.receivables ?? []} /></div>
          </div>
        </>
      )}

      <div className="panel">
        <div className="panel-head"><h2>{Ic.db} Entity üzrə qeyd sayı</h2></div>
        <div style={{ padding: "12px 16px" }}><BarChart data={data?.entities ?? []} /></div>
      </div>

      <div className="panel">
        <div className="panel-head"><h2>{Ic.activity} Sinxronizasiya dinamikası (14 gün)</h2></div>
        <div style={{ padding: "12px 16px" }}><LineChart data={data?.daily ?? []} /></div>
      </div>

      <div className="panel">
        <div className="panel-head"><h2>{Ic.plug} Bağlantı statusları</h2></div>
        <table className="tbl">
          <thead>
            <tr><th>Bağlantı</th><th>Tip</th><th>Status</th><th>Son sinxronizasiya</th></tr>
          </thead>
          <tbody>
            {data?.connections.map((c) => (
              <tr key={c.name}>
                <td>{c.name}</td>
                <td>{c.type}</td>
                <td>
                  <span style={{ color: c.last_error ? RED : "#1b7f3b", fontWeight: 600 }}>
                    {c.last_error ? "xəta" : c.status}
                  </span>
                  {c.last_error && <div style={{ fontSize: 11, color: "#888" }}>{c.last_error}</div>}
                </td>
                <td>{c.last_sync || "—"}</td>
              </tr>
            ))}
            {!data?.connections.length && (
              <tr><td colSpan={4} style={{ color: "#888" }}>Bağlantı yoxdur</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </>
  );
}
