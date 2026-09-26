import { useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import { Ic } from "./Icons";

interface Step { entity: string; status: "pending" | "running" | "done" | "error"; records: number; message?: string }
interface JobState { status: "idle" | "running" | "done" | "error"; steps?: Step[]; percent?: number }

// Polls the background 1C full-sync job and renders a step progress panel.
export default function C1SyncPanel({ connId, name, onClose }: { connId: string; name: string; onClose: () => void }) {
  const qc = useQueryClient();
  const { data } = useQuery({
    queryKey: ["c1sync", connId],
    queryFn: async () => (await api.get<JobState>(`/connections/${connId}/c1sync`)).data,
    refetchInterval: (q) => {
      const st = (q.state.data as JobState | undefined)?.status;
      return st === "done" || st === "error" ? false : 1200;
    },
  });
  const finished = data?.status === "done" || data?.status === "error";
  const notified = useRef(false);
  useEffect(() => {
    if (finished && !notified.current) {
      notified.current = true;
      qc.invalidateQueries({ queryKey: ["connections"] });
      qc.invalidateQueries({ queryKey: ["data-summary"] });
    }
  }, [finished, qc]);

  const pct = finished ? 100 : data?.percent ?? 0;
  const stIcon = (s: Step["status"]) =>
    s === "done" ? <span style={{ color: "#16a34a", fontWeight: 700 }}>✓</span>
    : s === "error" ? <span style={{ color: "#c62828", fontWeight: 700 }}>✗</span>
    : s === "running" ? <span className="spin-dot">●</span>
    : <span style={{ color: "#c6c9d4" }}>○</span>;

  return (
    <div className="panel" style={{ marginTop: "1rem" }}>
      <div className="panel-head" style={{ paddingBottom: ".4rem" }}>
        <h2>{Ic.refresh} Tam sinxronizasiya — {name}</h2>
        {finished && <button className="ibtn" title="Bağla" onClick={onClose}>✕</button>}
      </div>
      <div style={{ padding: "0 1.2rem 1rem" }}>
        <div className="pbar"><i style={{ width: `${pct}%` }} /></div>
        <div style={{ display: "flex", gap: "1.6rem", flexWrap: "wrap", marginTop: ".7rem" }}>
          {data?.steps?.map((s) => (
            <span key={s.entity} style={{ fontSize: ".86rem", display: "flex", alignItems: "center", gap: ".4rem" }}>
              {stIcon(s.status)} {s.entity}
              {s.status === "done" && <b>{s.records.toLocaleString("az")}</b>}
              {s.status === "error" && <em style={{ color: "#c62828", fontStyle: "normal" }}>{s.message}</em>}
            </span>
          ))}
        </div>
        {data?.status === "done" && <p className="ok-msg" style={{ marginTop: ".6rem" }}>✓ Sinxronizasiya tamamlandı — data Data mərkəzindədir</p>}
        {data?.status === "error" && <p className="error" style={{ marginTop: ".6rem" }}>Sinxronizasiya xəta ilə bitdi</p>}
      </div>
    </div>
  );
}
