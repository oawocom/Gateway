import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import Modal from "../../components/Modal";

interface Auto {
  id: string; name: string; connection_id: string; connection_name: string;
  entity_name: string; interval_minutes: number; enabled: boolean;
  last_run_at: string | null; last_status: string | null;
}
interface Conn { id: string; name: string }

export default function Automations() {
  const me = useAuth((s) => s.user);
  const qc = useQueryClient();
  const [show, setShow] = useState(false);
  const [name, setName] = useState("");
  const [connId, setConnId] = useState("");
  const [entity, setEntity] = useState("");
  const [interval, setIntervalMin] = useState(60);
  const [error, setError] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["automations"],
    queryFn: async () => (await api.get<{ automations: Auto[] }>("/automations")).data.automations,
    refetchInterval: 30000,
  });

  const conns = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await api.get<{ connections: Conn[] }>("/connections")).data.connections,
  });

  const entities = useQuery({
    queryKey: ["entities", connId],
    queryFn: async () => (await api.get<{ entities: string[] }>(`/connections/${connId}/entities`)).data.entities,
    enabled: !!connId && show,
  });

  const canManage = me?.role === "owner" || me?.role === "admin";
  const invalidate = () => qc.invalidateQueries({ queryKey: ["automations"] });

  const create = useMutation({
    mutationFn: () => api.post("/automations", {
      name, connection_id: connId, entity_name: entity, interval_minutes: interval,
    }),
    onSuccess: () => { invalidate(); setShow(false); setName(""); setEntity(""); setError(""); },
    onError: (e: any) => setError(e.response?.data?.error ?? "Xəta baş verdi"),
  });

  const toggle = useMutation({
    mutationFn: (a: Auto) => api.patch(`/automations/${a.id}`, { enabled: !a.enabled }),
    onSuccess: invalidate,
  });

  const del = useMutation({
    mutationFn: (id: string) => api.delete(`/automations/${id}`),
    onSuccess: invalidate,
  });

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Avtomatlaşdırma</h1>
          <p className="page-sub">Planlaşdırılmış sinxronizasiyalar</p>
        </div>
        {canManage && <button className="btn-primary" onClick={() => setShow(true)}>+ Yeni avtomatlaşdırma</button>}
      </div>

      {isLoading ? <p>Yüklənir...</p> : !data?.length ? (
        <div className="empty-state">
          <h3>Hələ avtomatlaşdırma yoxdur</h3>
          <p>Datanın müəyyən intervalla avtomatik sinxronlaşmasını qurun.</p>
        </div>
      ) : (
        <table className="tbl">
          <thead>
            <tr><th>Ad</th><th>Bağlantı</th><th>Entity</th><th>İnterval</th><th>Son icra</th><th>Status</th><th></th></tr>
          </thead>
          <tbody>
            {data.map((a) => (
              <tr key={a.id}>
                <td>{a.name}</td>
                <td>{a.connection_name}</td>
                <td>{a.entity_name}</td>
                <td>{a.interval_minutes} dəq</td>
                <td>{a.last_run_at ? new Date(a.last_run_at).toLocaleString("az") : "—"}</td>
                <td>
                  <span className={`badge ${a.enabled ? (a.last_status === "error" ? "suspended" : "active") : "provisioning"}`}>
                    {!a.enabled ? "dayandırılıb" : a.last_status ?? "gözləyir"}
                  </span>
                </td>
                <td className="row-actions">
                  {canManage && <>
                    <button onClick={() => toggle.mutate(a)}>{a.enabled ? "Dayandır" : "Aktivləşdir"}</button>
                    <button className="btn-danger" onClick={() => { if (confirm("Silinsin?")) del.mutate(a.id); }}>Sil</button>
                  </>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {show && (
        <Modal title="Yeni avtomatlaşdırma" onClose={() => setShow(false)}>
          <label>Ad</label>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="məs. Nomenklatura sync" />
          <label>Bağlantı</label>
          <select value={connId} onChange={(e) => { setConnId(e.target.value); setEntity(""); }}>
            <option value="">Seçin...</option>
            {conns.data?.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
          <label>Entity</label>
          <select value={entity} onChange={(e) => setEntity(e.target.value)} disabled={!connId}>
            <option value="">{entities.isLoading ? "Yüklənir..." : "Seçin..."}</option>
            {entities.data?.map((e) => <option key={e} value={e}>{e}</option>)}
          </select>
          <label>İnterval (dəqiqə, min 5)</label>
          <input type="number" min={5} value={interval} onChange={(e) => setIntervalMin(parseInt(e.target.value) || 5)} />
          {error && <p className="error">{error}</p>}
          <div className="modal-actions">
            <button onClick={() => setShow(false)}>Ləğv et</button>
            <button className="btn-primary" disabled={create.isPending || !name || !connId || !entity}
              onClick={() => create.mutate()}>Yarat</button>
          </div>
        </Modal>
      )}
    </>
  );
}
