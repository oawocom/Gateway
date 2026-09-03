import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router-dom";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import Modal from "../../components/Modal";
import { Ic, connMeta } from "../../components/Icons";
import PasswordInput from "../../components/PasswordInput";

interface Field { key: string; label: string; type: string; required: boolean; hint?: string }
interface Connector {
  type: string; name: string; description: string;
  category: string; available: boolean; fields?: Field[];
}

export default function Integrations() {
  const me = useAuth((s) => s.user);
  const nav = useNavigate();
  const qc = useQueryClient();
  const [cat, setCat] = useState("Hamısı");
  const [selected, setSelected] = useState<Connector | null>(null);
  const [name, setName] = useState("");
  const [config, setConfig] = useState<Record<string, string>>({});
  const [testMsg, setTestMsg] = useState<{ ok: boolean; message: string } | null>(null);
  const [error, setError] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["connectors"],
    queryFn: async () => (await api.get<{ connectors: Connector[] }>("/connectors")).data.connectors,
  });

  const cats = ["Hamısı", ...new Set(data?.map((c) => c.category) ?? [])];
  const shown = data?.filter((c) => cat === "Hamısı" || c.category === cat);
  const canManage = me?.role === "owner" || me?.role === "admin";

  const test = useMutation({
    mutationFn: () => api.post("/connections/test", { connector_type: selected!.type, config }),
    onSuccess: (r) => setTestMsg(r.data),
    onError: (e: any) => setTestMsg({ ok: false, message: e.response?.data?.error ?? "Xəta" }),
  });

  const create = useMutation({
    mutationFn: () => api.post("/connections", { name, connector_type: selected!.type, config }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["connections"] });
      setSelected(null);
      nav("/connections");
    },
    onError: (e: any) => setError(e.response?.data?.error ?? "Xəta baş verdi"),
  });

  function open(c: Connector) {
    setSelected(c); setName(c.name); setConfig({}); setTestMsg(null); setError("");
  }

  return (
    <>
      <h1>Marketplace</h1>
      <p className="page-sub">Sistemlərinizi Gateway-ə qoşun — hamısı bir mərkəzdə</p>

      <div className="chips">
        {cats.map((c) => (
          <button key={c} className={`chip ${cat === c ? "on" : ""}`} onClick={() => setCat(c)}>{c}</button>
        ))}
      </div>

      {isLoading ? <p>Yüklənir...</p> : (
        <div className="grid-cards">
          {shown?.map((c) => {
            const meta = connMeta[c.type] ?? { icon: "🔌", tint: "#f2f3f8" };
            return (
              <div key={c.type} className="connector-card">
                <div className="cc-top">
                  <span className="cc-tile" style={{ background: meta.tint }}>{meta.icon}</span>
                  <div>
                    <strong>{c.name}</strong>
                    <span className="cc-cat">{c.category}</span>
                  </div>
                  {c.available && <span className="cc-pop">Aktiv</span>}
                </div>
                <p>{c.description}</p>
                {c.available ? (
                  canManage
                    ? <button className="btn-primary w-full" onClick={() => open(c)}>{Ic.plus} Qoş</button>
                    : <span className="cc-soon">Yalnız admin qoşa bilər</span>
                ) : (
                  <span className="cc-soon">{Ic.clock} Tezliklə</span>
                )}
              </div>
            );
          })}
        </div>
      )}

      {selected && (
        <Modal title={`${selected.name} — yeni bağlantı`} onClose={() => setSelected(null)}>
          <div className="wizard-steps">
            <span className="ws on">1. Məlumatlar</span>
            <span className={`ws ${testMsg?.ok ? "on" : ""}`}>2. Test</span>
            <span className="ws">3. Hazır</span>
          </div>
          <label>Bağlantı adı</label>
          <input value={name} onChange={(e) => setName(e.target.value)} />
          {selected.fields?.map((f) => (
            <div key={f.key} className="field">
              <label>{f.label}</label>
              {f.type === "password" ? (
                <PasswordInput
                  placeholder={f.hint ?? ""}
                  value={config[f.key] ?? ""}
                  onChange={(v) => setConfig({ ...config, [f.key]: v })}
                />
              ) : f.type === "textarea" ? (
                <textarea
                  className="cfg-area"
                  rows={4}
                  placeholder={f.hint ?? ""}
                  value={config[f.key] ?? ""}
                  onChange={(e) => setConfig({ ...config, [f.key]: e.target.value })}
                />
              ) : (
                <input
                  type="text"
                  placeholder={f.hint ?? ""}
                  value={config[f.key] ?? ""}
                  onChange={(e) => setConfig({ ...config, [f.key]: e.target.value })}
                />
              )}
            </div>
          ))}
          {testMsg && <p className={testMsg.ok ? "ok-msg" : "error"}>{testMsg.ok ? "✓ " : ""}{testMsg.message}</p>}
          {error && <p className="error">{error}</p>}
          <div className="modal-actions">
            <button onClick={() => test.mutate()} disabled={test.isPending}>
              {Ic.test} {test.isPending ? "Yoxlanılır..." : "Test et"}
            </button>
            <button className="btn-primary" onClick={() => create.mutate()} disabled={create.isPending || !testMsg?.ok}>
              {Ic.check} {create.isPending ? "Yadda saxlanılır..." : "Yadda saxla"}
            </button>
          </div>
        </Modal>
      )}
    </>
  );
}
