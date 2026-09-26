import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import Modal from "../../components/Modal";
import C1SyncPanel from "../../components/C1SyncPanel";
import { Ic, connMeta } from "../../components/Icons";
import ConnectModal, { cardTypeOf } from "../../components/ConnectModal";
import C1AggSettings from "../../components/C1AggSettings";
import type { Connector } from "../../components/ConnectModal";

interface Conn {
  id: string; name: string; connector_type: string; source?: string; status: string;
  last_sync_at: string | null; last_error: string | null; created_at: string;
}

export default function ConnectorDetail() {
  const { type = "" } = useParams();
  const me = useAuth((s) => s.user);
  const qc = useQueryClient();
  const [adding, setAdding] = useState(false);
  const [entConn, setEntConn] = useState<Conn | null>(null);
  const [cfgConn, setCfgConn] = useState<Conn | null>(null);
  const [syncing, setSyncing] = useState<string | null>(null);
  const [msg, setMsg] = useState("");

  const { data: catalog } = useQuery({
    queryKey: ["connectors"],
    queryFn: async () => (await api.get<{ connectors: Connector[] }>("/connectors")).data.connectors,
  });
  const { data: conns, isLoading } = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await api.get<{ connections: Conn[] }>("/connections")).data.connections,
  });

  const connector = catalog?.find((c) => c.type === type);
  const mine = (conns ?? []).filter((c) => cardTypeOf(c) === type);
  const canManage = me?.role === "owner" || me?.role === "admin";
  const meta = connMeta[type] ?? { icon: "🔌", tint: "#f2f3f8" };

  const [editing, setEditing] = useState<{ id: string; name: string } | null>(null);
  const renameMut = useMutation({
    mutationFn: (v: { id: string; name: string }) => api.patch(`/connections/${v.id}`, { name: v.name }),
    onSuccess: () => {
      setEditing(null);
      qc.invalidateQueries({ queryKey: ["connections"] });
      qc.invalidateQueries({ queryKey: ["c1-menu"] });
    },
    onError: (e: any) => setMsg("✗ " + (e.response?.data?.error ?? "Ad dəyişmədi")),
  });
  const testMut = useMutation({
    mutationFn: (id: string) => api.post(`/connections/${id}/test`),
    onSuccess: (r) => setMsg(r.data.ok ? "✓ " + r.data.message : "✗ " + r.data.message),
  });
  const delMut = useMutation({
    mutationFn: (id: string) => api.delete(`/connections/${id}`),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["connections"] });
      qc.invalidateQueries({ queryKey: ["c1-menu"] });
    },
  });
  const entities = useQuery({
    queryKey: ["entities", entConn?.id],
    queryFn: async () => (await api.get<{ entities: string[] }>(`/connections/${entConn!.id}/entities`)).data.entities,
    enabled: !!entConn,
  });


  const [syncConn, setSyncConn] = useState<Conn | null>(null);
  async function fullSync(c: Conn) {
    try {
      await api.post(`/connections/${c.id}/c1sync`);
      setSyncConn(c);
    } catch (e: any) {
      setMsg("✗ " + (e.response?.data?.error ?? "Xəta"));
    }
  }


  const entLabel = (e: string) =>
    e === "accounts" ? "Hesablar və qalıqlar" :
    e.startsWith("statements:") ? `Çıxarış — ${e.slice(11)}` : e;

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

  if (!connector && catalog) {
    return <div className="empty-state"><h3>Connector tapılmadı</h3><Link className="btn-primary" to="/integrations">{Ic.store} İnteqrasiyalar</Link></div>;
  }

  return (
    <>
      <p className="page-sub"><Link to="/integrations" style={{ color: "#8b90a3" }}>İnteqrasiyalar</Link> / {connector?.name ?? type}</p>
      <div className="page-head">
        <div style={{ display: "flex", gap: ".9rem", alignItems: "center" }}>
          <span className="cc-tile" style={{ background: meta.tint }}>{meta.icon}</span>
          <div>
            <h1 style={{ marginBottom: 0 }}>{connector?.name ?? type}</h1>
            <p className="page-sub" style={{ marginTop: ".15rem" }}>{connector?.description}</p>
          </div>
        </div>
        {canManage && connector?.available && (
          <button className="btn-primary" onClick={() => setAdding(true)}>{Ic.plus} Yeni bağlantı</button>
        )}
      </div>

      {msg && <p className="info-bar">{msg}</p>}
      {syncConn && <C1SyncPanel connId={syncConn.id} name={syncConn.name} onClose={() => setSyncConn(null)} />}

      {isLoading ? <p>Yüklənir...</p> : !mine.length ? (
        <div className="empty-state">
          <h3>Bu inteqrasiya üzrə bağlantı yoxdur</h3>
          <p>İlk bağlantını yaradın — hesabatlar və data avtomatik açılacaq.</p>
          {canManage && connector?.available && (
            <button className="btn-primary" onClick={() => setAdding(true)}>{Ic.plus} Yeni bağlantı</button>
          )}
        </div>
      ) : (
        <div className="conn-list">
          {mine.map((c) => (
            <div key={c.id} className="conn-row">
              <span className="cc-tile sm" style={{ background: meta.tint }}>{meta.icon}</span>
              <div className="conn-main">
                {editing?.id === c.id ? (
                  <span style={{ display: "flex", gap: ".4rem", alignItems: "center" }}>
                    <input autoFocus value={editing.name}
                      onChange={(e) => setEditing({ id: c.id, name: e.target.value })}
                      onKeyDown={(e) => { if (e.key === "Enter" && editing.name.trim()) renameMut.mutate(editing); if (e.key === "Escape") setEditing(null); }}
                      style={{ padding: ".3rem .5rem", border: "1px solid #f0c000", fontWeight: 600 }} />
                    <button className="ibtn gold" title="Yadda saxla" disabled={!editing.name.trim() || renameMut.isPending}
                      onClick={() => renameMut.mutate(editing)}>{Ic.check}</button>
                    <button className="ibtn" title="İmtina" onClick={() => setEditing(null)}>✕</button>
                  </span>
                ) : <strong>{c.name}</strong>}
                <span>
                  <em className={`dot ${c.last_error ? "error" : "success"}`} />
                  {c.last_error ? "xəta" : "aktiv"}
                  {c.last_sync_at && ` · son sinxronizasiya ${new Date(c.last_sync_at).toLocaleString("az")}`}
                </span>
                {c.last_error && <span className="conn-err">{c.last_error}</span>}
              </div>
              {canManage && (
                <div className="icon-actions">
                  {c.source === "1c" && c.connector_type === "mssql" && (
                    <button className="ibtn" title="1C parametrləri" onClick={() => setCfgConn(c)}>⚙</button>
                  )}
                  <button className="ibtn" title="Adı dəyiş" onClick={() => setEditing({ id: c.id, name: c.name })}>✎</button>
                  <button className="ibtn" title="Test et" disabled={testMut.isPending} onClick={() => testMut.mutate(c.id)}>{Ic.test}</button>
                  <button className="ibtn gold" title={c.source === "1c" && c.connector_type === "mssql" ? "Tam sinxronizasiya" : "Sinxronizasiya"}
                      onClick={() => (c.source === "1c" && c.connector_type === "mssql") ? fullSync(c) : setEntConn(c)}>
                      {Ic.refresh}</button>
                  <button className="ibtn warn" title="Sil" onClick={() => { if (confirm(`${c.name} silinsin? Bütün sinxronlaşmış data silinəcək.`)) delMut.mutate(c.id); }}>{Ic.trash}</button>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {adding && connector && catalog && (
        <ConnectModal connector={connector} all={catalog} onClose={() => setAdding(false)} />
      )}

      {cfgConn && <C1AggSettings connId={cfgConn.id} name={cfgConn.name} onClose={() => setCfgConn(null)} />}

      {entConn && (
        <Modal title={`${entConn.name} — entity seçin`} onClose={() => setEntConn(null)} wide>
          {entities.isLoading ? <p>Entity siyahısı alınır...</p> :
           entities.isError ? <p className="error">{(entities.error as any)?.response?.data?.error ?? "Siyahı alına bilmədi"}</p> : (
            <><div>
              <button className="btn-primary sm" style={{ marginBottom: ".6rem" }} disabled={syncing !== null}
                onClick={async () => { for (const e of entities.data ?? []) { await syncEntity(e); } }}>
                {Ic.refresh} Hamısını sinxronlaşdır
              </button>
            </div>
            <div className="entity-list">
              {entities.data?.map((e) => (
                <div key={e} className="entity-row">
                  <span>{entLabel(e)}</span>
                  <button className="btn-primary sm" disabled={syncing !== null} onClick={() => syncEntity(e)}>
                    {Ic.refresh} {syncing === e ? "Sinxronlaşır..." : "Sinxronizasiya"}
                  </button>
                </div>
              ))}
            </div></>
          )}
        </Modal>
      )}
    </>
  );
}
