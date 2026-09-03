import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";

interface LogRow {
  id: number; connection_name: string; entity_name: string;
  status: string; message: string; records_synced: number; created_at: string;
}

export default function Settings() {
  const { tenant, user } = useAuth();
  const { data } = useQuery({
    queryKey: ["synclog"],
    queryFn: async () => (await api.get<{ log: LogRow[] }>("/data/synclog")).data.log,
  });

  return (
    <>
      <h1>Parametrlər</h1>
      <p className="page-sub">İş sahəsi məlumatları və sinxronizasiya jurnalı</p>

      <div className="cards" style={{ maxWidth: 720 }}>
        <div className="card"><span>İş sahəsi</span><strong style={{ fontSize: "1.1rem" }}>{tenant?.name}</strong></div>
        <div className="card"><span>Slug</span><strong style={{ fontSize: "1.1rem" }}>{tenant?.slug}</strong></div>
        <div className="card"><span>Rolunuz</span><strong style={{ fontSize: "1.1rem" }}>{user?.role}</strong></div>
      </div>

      <h2 style={{ marginTop: "2rem" }}>Sinxronizasiya jurnalı</h2>
      {!data?.length ? <p className="page-sub">Jurnal boşdur.</p> : (
        <table className="tbl">
          <thead><tr><th>Vaxt</th><th>Bağlantı</th><th>Entity</th><th>Status</th><th>Qeyd</th><th>Mesaj</th></tr></thead>
          <tbody>
            {data.map((l) => (
              <tr key={l.id}>
                <td>{new Date(l.created_at).toLocaleString("az")}</td>
                <td>{l.connection_name}</td>
                <td>{l.entity_name}</td>
                <td><span className={`badge ${l.status === "success" ? "active" : "suspended"}`}>{l.status}</span></td>
                <td>{l.records_synced}</td>
                <td className="msg-cell">{l.message}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}
