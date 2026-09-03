import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";

export interface Agent {
  id: string; name: string; description: string;
  system_prompt?: string; enabled: boolean; updated_at: string;
}

export default function AgentList() {
  const nav = useNavigate();
  const qc = useQueryClient();
  const [q, setQ] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["admin-agents"],
    queryFn: async () => (await api.get<{ agents: Agent[] }>("/admin/agents")).data.agents,
  });

  const toggle = useMutation({
    mutationFn: (a: Agent) => api.patch(`/admin/agents/${a.id}`, { enabled: !a.enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-agents"] }),
  });

  const del = useMutation({
    mutationFn: (id: string) => api.delete(`/admin/agents/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-agents"] }),
  });

  const shown = data?.filter((a) =>
    !q || a.name.toLowerCase().includes(q.toLowerCase()) || a.description.toLowerCase().includes(q.toLowerCase()));

  return (
    <>
      <div className="page-head">
        <div>
          <h1>AI Agentlər</h1>
          <p className="page-sub">Bütün tenantların istifadə edəcəyi qlobal agentlər</p>
        </div>
        <div className="head-tools">
          <div className="search-box">
            {Ic.search}
            <input placeholder="Axtar..." value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <Link className="btn-primary" to="/admin/agents/new">{Ic.plus} Əlavə et</Link>
        </div>
      </div>

      {isLoading ? <p>Yüklənir...</p> : !data?.length ? (
        <div className="empty-state">
          <h3>Hələ agent yoxdur</h3>
          <p>İlk qlobal AI agenti yaradın — bütün tenantlar ondan istifadə edə biləcək.</p>
          <Link className="btn-primary" to="/admin/agents/new">{Ic.plus} Agent yarat</Link>
        </div>
      ) : (
        <div className="tenant-list agent-grid" style={{ marginTop: "1.1rem" }}>
          <div className="tl-head agent-cols">
            <span>Agent</span><span>Təsvir</span><span>Yenilənib</span><span>Status</span><span className="ta-r">Əməliyyatlar</span>
          </div>
          {shown?.map((a) => (
            <div key={a.id} className="tl-row agent-cols" onClick={() => nav(`/admin/agents/${a.id}`)}>
              <span className="tl-name agent-name"><i className="agent-ic">{Ic.bot}</i>{a.name}</span>
              <span className="tl-desc">{a.description || "—"}</span>
              <span>{new Date(a.updated_at).toLocaleDateString("az")}</span>
              <span className="switch-wrap" onClick={(e) => e.stopPropagation()}>
                <button type="button" className={`switch ${a.enabled ? "on" : ""}`}
                  disabled={toggle.isPending} onClick={() => toggle.mutate(a)} />
                <em className={a.enabled ? "sw-on" : "sw-off"}>{a.enabled ? "aktiv" : "deaktiv"}</em>
              </span>
              <span className="icon-actions" onClick={(e) => e.stopPropagation()}>
                <button className="ibtn" title="Redaktə et" onClick={() => nav(`/admin/agents/${a.id}`)}>{Ic.edit}</button>
                <button className="ibtn warn" title="Sil"
                  onClick={() => { if (confirm(`"${a.name}" silinsin?`)) del.mutate(a.id); }}>{Ic.trash}</button>
              </span>
            </div>
          ))}
          {!shown?.length && <div className="expand-pad">Nəticə tapılmadı.</div>}
        </div>
      )}
    </>
  );
}
