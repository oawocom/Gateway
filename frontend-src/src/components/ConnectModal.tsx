import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import Modal from "./Modal";
import { Ic } from "./Icons";
import PasswordInput from "./PasswordInput";

export interface Field { key: string; label: string; type: string; required: boolean; hint?: string }
export interface Connector {
  type: string; name: string; description: string;
  category: string; available: boolean; hidden?: boolean; fields?: Field[];
}

// 1C wizard: the customer picks the version/configuration; the connection
// method (and its fields) follows from that choice. Field sets come from the
// hidden catalog descriptors, so there is a single source of truth.
export const C1_VERSIONS = [
  { id: "azstandart", method: "mssql", title: "AzStandart — Бухгалтерия для Азербайджана", sub: "8.2 / 8.3 · MSSQL bazasına birbaşa qoşulma", name: "1C (AzStandart)" },
  { id: "odata", method: "1c_odata", title: "1C 8.3.5 və yuxarı", sub: "Standart OData publikasiyası ilə", name: "1C (OData)" },
  { id: "other", method: "1c_http", title: "Digər / köhnə versiya", sub: "Bazada publikasiya olunmuş HTTP servislərlə", name: "1C (HTTP Servis)" },
];

export default function ConnectModal({ connector, all, onClose, onCreated }: {
  connector: Connector;          // the card the user picked (may be the virtual "1c")
  all: Connector[];              // full catalog incl. hidden, for wizard field sets
  onClose: () => void;
  onCreated?: () => void;
}) {
  const qc = useQueryClient();
  const [name, setName] = useState(connector.name);
  const [config, setConfig] = useState<Record<string, string>>({});
  const [testMsg, setTestMsg] = useState<{ ok: boolean; message: string } | null>(null);
  const [error, setError] = useState("");
  const [c1ver, setC1ver] = useState("");

  const isC1 = connector.type === "1c";
  const c1choice = C1_VERSIONS.find((v) => v.id === c1ver);
  const effType = isC1 ? c1choice?.method ?? "" : connector.type;
  const effFields = isC1 ? (c1choice ? all.find((c) => c.type === c1choice.method)?.fields : undefined) : connector.fields;

  const test = useMutation({
    mutationFn: () => api.post("/connections/test", { connector_type: effType, config }),
    onSuccess: (r) => setTestMsg(r.data),
    onError: (e: any) => setTestMsg({ ok: false, message: e.response?.data?.error ?? "Xəta" }),
  });

  const create = useMutation({
    mutationFn: () => api.post<{ id: string }>("/connections", {
      name, connector_type: effType, config, source: isC1 ? "1c" : "",
    }),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: ["connections"] });
      if (isC1 && effType === "mssql") {
        // detect the 1C configuration in the background so the reports menu appears
        api.get(`/connections/${r.data.id}/c1meta`).finally(() =>
          qc.invalidateQueries({ queryKey: ["c1-menu"] }));
      } else {
        qc.invalidateQueries({ queryKey: ["c1-menu"] });
      }
      onCreated?.();
      onClose();
    },
    onError: (e: any) => setError(e.response?.data?.error ?? "Xəta baş verdi"),
  });

  function pickVersion(id: string) {
    const v = C1_VERSIONS.find((x) => x.id === id)!;
    setC1ver(id); setName(v.name); setConfig({}); setTestMsg(null); setError("");
  }

  return (
    <Modal title={`${connector.name} — yeni bağlantı`} onClose={onClose}>
      <div className="wizard-steps">
        {isC1 && <span className="ws on">1. Versiya</span>}
        <span className={`ws ${!isC1 || c1ver ? "on" : ""}`}>{isC1 ? "2" : "1"}. Məlumatlar</span>
        <span className={`ws ${testMsg?.ok ? "on" : ""}`}>{isC1 ? "3" : "2"}. Test</span>
        <span className="ws">{isC1 ? "4" : "3"}. Hazır</span>
      </div>
      {isC1 && (
        <div className="field">
          <label>1C versiyası / konfiqurasiyası</label>
          {C1_VERSIONS.map((v) => (
            <div key={v.id}
              onClick={() => pickVersion(v.id)}
              style={{
                border: `1px solid ${c1ver === v.id ? "#f0c000" : "#dfe2ea"}`,
                background: c1ver === v.id ? "#fdf8e4" : "#fff",
                padding: ".6rem .8rem", marginBottom: ".4rem", cursor: "pointer",
              }}>
              <strong style={{ display: "block", fontSize: ".92rem" }}>{v.title}</strong>
              <span style={{ fontSize: ".8rem", color: "#8b90a3" }}>{v.sub}</span>
            </div>
          ))}
        </div>
      )}
      {(!isC1 || c1ver) && (<>
        <label>Bağlantı adı <span style={{ color: "#c62828" }}>*</span></label>
        <input value={name} onChange={(e) => setName(e.target.value)}
          placeholder="məs. AZFIBERNET 1C" />
        <small style={{ color: "#8b90a3", display: "block", marginTop: ".2rem" }}>
          Bu ad sol menyuda hesabat bölməsinin başlığı olacaq
        </small>
        {effFields?.map((f) => (
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
          <button className="btn-primary" onClick={() => create.mutate()} disabled={create.isPending || !testMsg?.ok || !name.trim()}>
            {Ic.check} {create.isPending ? "Yadda saxlanılır..." : "Yadda saxla"}
          </button>
        </div>
      </>)}
    </Modal>
  );
}

// Which integration card a connection belongs to: wizard-created and
// 1C-native connections belong to "1c"; everything else to its own type.
export function cardTypeOf(c: { connector_type: string; source?: string }): string {
  if (c.source === "1c" || c.connector_type === "1c_odata" || c.connector_type === "1c_http") return "1c";
  return c.connector_type;
}
