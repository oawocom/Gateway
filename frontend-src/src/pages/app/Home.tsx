import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import { Ic } from "../../components/Icons";

interface Stats {
  connections: number; automations: number;
  syncs_this_month: number; errors_this_month: number; total_records: number;
}
interface LogRow {
  id: number; connection_name: string; entity_name: string;
  status: string; message: string; records_synced: number; created_at: string;
}

export default function Home() {
  const tenant = useAuth((s) => s.tenant);
  const { data } = useQuery({
    queryKey: ["stats"],
    queryFn: async () => (await api.get<Stats>("/stats")).data,
    refetchInterval: 30000,
  });
  const log = useQuery({
    queryKey: ["synclog"],
    queryFn: async () => (await api.get<{ log: LogRow[] }>("/data/synclog")).data.log,
    refetchInterval: 30000,
  });

  const stats = [
    { icon: Ic.plug, label: "Aktiv bağlantılar", val: data?.connections },
    { icon: Ic.zap, label: "Avtomatlaşdırmalar", val: data?.automations },
    { icon: Ic.refresh, label: "Bu ay sinxronizasiya", val: data?.syncs_this_month },
    { icon: Ic.alert, label: "Xətalar (bu ay)", val: data?.errors_this_month },
    { icon: Ic.db, label: "Ümumi qeydlər", val: data?.total_records },
  ];

  return (
    <>
      <h1>İdarə paneli</h1>
      <p className="page-sub">{tenant?.name} iş sahəsinin ümumi görünüşü</p>
      <div className="cards">
        {stats.map((s) => (
          <div key={s.label} className="card stat">
            <i>{s.icon}</i>
            <div><span>{s.label}</span><strong>{s.val ?? 0}</strong></div>
          </div>
        ))}
      </div>

      {(data?.connections ?? 0) === 0 ? (
        <div className="empty-state">
          <h3>Başlamaq üçün ilk inteqrasiyanı qoşun</h3>
          <p>1C, Zoho, iiko və digər sistemlərinizi vahid mərkəzdə birləşdirin.</p>
          <Link className="btn-primary" to="/integrations">{Ic.store} Marketplace-ə keç</Link>
        </div>
      ) : (
        <div className="panel">
          <div className="panel-head">
            <h2>{Ic.activity} Son aktivlik</h2>
            <Link to="/settings" className="mini-link">Hamısına bax {Ic.chevR}</Link>
          </div>
          {!log.data?.length ? <p className="page-sub" style={{ padding: "0 1.2rem 1.2rem" }}>Hələ aktivlik yoxdur.</p> : (
            <div className="feed">
              {log.data.slice(0, 8).map((l) => (
                <div key={l.id} className="feed-row">
                  <span className={`dot ${l.status}`} />
                  <span className="feed-main">
                    {l.connection_name} · {l.entity_name}
                    {l.status === "success" ? ` — ${l.records_synced} qeyd sinxronlaşdı` : ` — ${l.message}`}
                  </span>
                  <span className="feed-time">{new Date(l.created_at).toLocaleTimeString("az", { hour: "2-digit", minute: "2-digit" })}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </>
  );
}
