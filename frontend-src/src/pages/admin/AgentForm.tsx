import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";
import type { Agent } from "./AgentList";

export default function AgentForm() {
  const { id } = useParams();
  const isNew = !id;
  const nav = useNavigate();
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [prompt, setPrompt] = useState("");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);

  const agents = useQuery({
    queryKey: ["admin-agents"],
    queryFn: async () => (await api.get<{ agents: Agent[] }>("/admin/agents")).data.agents,
    enabled: !isNew,
  });
  const agent = agents.data?.find((a) => a.id === id);

  useEffect(() => {
    if (agent) {
      setName(agent.name);
      setDescription(agent.description);
      setPrompt(agent.system_prompt ?? "");
    }
  }, [agent]);

  const save = useMutation({
    mutationFn: () =>
      isNew
        ? api.post("/admin/agents", { name, description, system_prompt: prompt })
        : api.patch(`/admin/agents/${id}`, { name, description, system_prompt: prompt }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-agents"] });
      if (isNew) nav("/admin/agents");
      else { setSaved(true); setTimeout(() => setSaved(false), 2500); }
    },
    onError: (e: any) => setError(e.response?.data?.error ?? "Xəta baş verdi"),
  });

  if (!isNew && agents.isLoading) return <p>Yüklənir...</p>;
  if (!isNew && !agent) return <><Link to="/admin/agents" className="mini-link back">← AI Agentlər</Link><p>Agent tapılmadı.</p></>;

  return (
    <>
      <Link to="/admin/agents" className="mini-link back">← AI Agentlər</Link>
      <div className="page-head">
        <div>
          <h1>{isNew ? "Yeni agent" : agent!.name}</h1>
          <p className="page-sub">{isNew ? "Qlobal AI agent yaradın" : "Agent parametrlərini redaktə edin"}</p>
        </div>
        <button className="btn-primary" disabled={save.isPending || !name.trim()} onClick={() => save.mutate()}>
          {Ic.check} {save.isPending ? "Saxlanılır..." : saved ? "Yadda saxlandı" : "Yadda saxla"}
        </button>
      </div>

      <div className="panel form-wide" style={{ marginTop: "1.2rem" }}>
        <div className="fw-grid">
          <div className="field">
            <label>Agent adı</label>
            <input value={name} onChange={(e) => setName(e.target.value)} placeholder="məs. Satış Analitiki" />
          </div>
          <div className="field">
            <label>Təsvir</label>
            <input value={description} onChange={(e) => setDescription(e.target.value)}
              placeholder="Agent nə iş görür (tenantlara görünəcək)" />
          </div>
        </div>
        <div className="field" style={{ marginTop: ".9rem" }}>
          <label>System prompt</label>
          <textarea className="prompt-area" value={prompt} onChange={(e) => setPrompt(e.target.value)}
            placeholder="Agentin davranışını müəyyən edən təlimat..." rows={14} />
          <span className="hint">{prompt.length} simvol</span>
        </div>
        {error && <p className="error">{error}</p>}
      </div>
    </>
  );
}
