import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import Modal from "../../components/Modal";
import { Ic, connMeta } from "../../components/Icons";

interface Conn {
  id: string; name: string; connector_type: string; status: string;
  last_sync_at: string | null; last_error: string | null; created_at: string;
}

export default function Connections() {
  const me = useAuth((s) => s.user);
  const qc = useQueryClient();
  const [entConn, setEntConn] = useState<Conn | null>(null);
  const [syncing, setSyncing] = useState<string | null>(null);
  const [msg, setMsg] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await api.get<{ connections: Conn[] }>("/connections")).data.connections,
  });

  const entities = useQuery({
    queryKey: ["entities", entConn?.id],
    queryFn: async () => (await api.get<{ entities: string[] }>(`/connections/${entConn!.id}/entities`)).data.entities,
    enabled: !!entConn,
  });

  const canManage = me?.role === "owner" || me?.role === "admin";

  const testMut = useMutation({
    mutationFn: (id: string) => api.post(`/connections/${id}/test`),
    onSuccess: (r) => setMsg(r.data.ok ? "✓ " + r.data.message : "✗ " + r.data.message),
  });

  const delMut = useMutation({
    mutationFn: (id: string) => api.delete(`/connections/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["connections"] }),
  });

  async function syncEntity(entity: string) {
    if (!entConn) return;
    setSyncing(entity);
    try {
      const { data: r } = await api.post(`/connections/${entConn.id}/sync`, { entity });
      setMsg(r.ok ? `✓ ${entity}: ${r.records} qeyd sinxronlaşdı` : `✗ ${entity}: ${r.message}`);
      qc.invalidateQueries({ queryKey: ["connections"] });
      qc.invalidateQueries({ queryKey: ["data-summary"] });
    } catch (e: any) {
      setMsg("✗ " + (e.response?.data?.error ?? "Xəta"));
    } finally {
      setSyncing(null);
    }
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Bağlantılarım</h1>
          <p className="page-sub">Qoşulmuş sistemlər və sinxronizasiya</p>
        </div>
        {canManage && <Link className="btn-primary" to="/integrations">{Ic.plus} Yeni bağlantı</Link>}
      </div>
      {msg && <p className="info-bar">{msg}</p>}
      {isLoading ? <p>Yüklənir...</p> : !data?.length ? (
        <div className="empty-state">
          <h3>Hələ bağlantı yoxdur</h3>
          <p>Marketplace-dən ilk sisteminizi qoşun.</p>
          <Link className="btn-primary" to="/integrations">{Ic.store} Marketplace</Link>
        </div>
      ) : (
        <div className="conn-list">
          {data.map((c) => {
            const meta = connMeta[c.connector_type] ?? { icon: "🔌", tint: "#f2f3f8" };
            return (
              <div key={c.id} className="conn-row">
                <span className="cc-tile sm" style={{ background: meta.tint }}>{meta.icon}</span>
                <div className="conn-main">
                  <strong>{c.name}</strong>
                  <span>
                    <em className={`dot ${c.last_error ? "error" : "success"}`} />
                    {c.last_error ? "xəta" : "aktiv"}
                    {c.last_sync_at && ` · son sinxronizasiya ${new Date(c.last_sync_at).toLocaleString("az")}`}
                  </span>
                  {c.last_error && <span className="conn-err">{c.last_error}</span>}
                </div>
                {canManage && (
                  <div className="icon-actions">
                    <button className="ibtn" title="Test et" disabled={testMut.isPending} onClick={() => testMut.mutate(c.id)}>{Ic.test}</button>
                    <button className="ibtn gold" title="Sinxronizasiya" onClick={() => setEntConn(c)}>{Ic.refresh}</button>
                    <button className="ibtn warn" title="Sil" onClick={() => { if (confirm(`${c.name} silinsin? Bütün sinxronlaşmış data silinəcək.`)) delMut.mutate(c.id); }}>{Ic.trash}</button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {entConn && (
        <Modal title={`${entConn.name} — entity seçin`} onClose={() => setEntConn(null)} wide>
          {entities.isLoading ? <p>1C-dən entity siyahısı alınır...</p> :
           entities.isError ? <p className="error">{(entities.error as any)?.response?.data?.error ?? "Siyahı alına bilmədi"}</p> : (
            <div className="entity-list">
              {entities.data?.map((e) => (
                <div key={e} className="entity-row">
                  <span>{e}</span>
                  <button className="btn-primary sm" disabled={syncing !== null} onClick={() => syncEntity(e)}>
                    {Ic.refresh} {syncing === e ? "Sinxronlaşır..." : "Sinxronizasiya"}
                  </button>
                </div>
              ))}
            </div>
          )}
        </Modal>
      )}
    </>
  );
}
