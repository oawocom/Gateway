import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import Modal from "../../components/Modal";
import PasswordInput from "../../components/PasswordInput";
import { Ic } from "../../components/Icons";

interface U {
  id: string; email: string; role: string; active: boolean;
  last_login_at: string | null; created_at: string;
}

const ROLE_AZ: Record<string, string> = { owner: "Sahib", admin: "Admin", member: "Analitik" };
const ROLE_DESC: Record<string, string> = {
  admin: "Bağlantılar, parametrlər və istifadəçi idarəsi",
  member: "Bütün hesabatlara baxış və Excel export — bağlantılara toxunmur",
};
const OK = "#16a34a", RED = "#c62828", GRAY = "#8b90a3";

export default function Users() {
  const me = useAuth((s) => s.user);
  const qc = useQueryClient();
  const [show, setShow] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("member");
  const [error, setError] = useState("");
  const [msg, setMsg] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["users"],
    queryFn: async () => (await api.get<{ users: U[] }>("/users")).data.users,
  });

  const invalidate = () => qc.invalidateQueries({ queryKey: ["users"] });
  const onErr = (e: any) => setMsg("✗ " + (e.response?.data?.error ?? "Xəta baş verdi"));

  const create = useMutation({
    mutationFn: () => api.post("/users", { email, password, role }),
    onSuccess: () => { invalidate(); setShow(false); setEmail(""); setPassword(""); setRole("member"); setError(""); },
    onError: (e: any) => setError(e.response?.data?.error ?? "Xəta baş verdi"),
  });
  const changeRole = useMutation({
    mutationFn: (p: { id: string; role: string }) => api.patch(`/users/${p.id}`, { role: p.role }),
    onSuccess: invalidate, onError: onErr,
  });
  const setActive = useMutation({
    mutationFn: (p: { id: string; active: boolean }) => api.patch(`/users/${p.id}/active`, { active: p.active }),
    onSuccess: invalidate, onError: onErr,
  });
  const remove = useMutation({
    mutationFn: (id: string) => api.delete(`/users/${id}`),
    onSuccess: invalidate, onError: onErr,
  });

  const isOwner = me?.role === "owner";
  // admins manage members only; owner manages everyone except themselves
  const canTouch = (u: U) =>
    u.id !== me?.id && u.role !== "owner" && (isOwner || u.role === "member");
  const dt = (s: string | null) => s ? new Date(s).toLocaleString("az") : "—";

  return (
    <>
      <div className="page-head">
        <div>
          <h1>İstifadəçilər</h1>
          <p className="page-sub">Rollar: Sahib — hər şey · Admin — bağlantılar və idarəetmə · Analitik — yalnız hesabatlar</p>
        </div>
        <button className="btn-primary" onClick={() => { setShow(true); setError(""); }}>{Ic.plus} İstifadəçi əlavə et</button>
      </div>

      {msg && <p className="info-bar">{msg}</p>}

      {isLoading ? <p>Yüklənir...</p> : (
        <table className="tbl">
          <thead><tr>
            <th>E-mail</th><th>Rol</th><th>Status</th><th>Son giriş</th><th>Yaradılıb</th><th></th>
          </tr></thead>
          <tbody>
            {data?.map((u) => (
              <tr key={u.id} style={{ opacity: u.active ? 1 : 0.55 }}>
                <td style={{ fontWeight: 600 }}>
                  {u.email}{u.id === me?.id && <span style={{ color: GRAY, fontWeight: 400 }}> (siz)</span>}
                </td>
                <td>
                  {canTouch(u) && isOwner ? (
                    <select value={u.role} disabled={changeRole.isPending}
                      onChange={(e) => changeRole.mutate({ id: u.id, role: e.target.value })}
                      style={{ padding: ".3rem .5rem", border: "1px solid #dfe2ea" }}>
                      <option value="admin">Admin</option>
                      <option value="member">Analitik</option>
                    </select>
                  ) : (
                    <span style={{
                      fontSize: ".78rem", fontWeight: 700, padding: ".2rem .5rem",
                      background: u.role === "owner" ? "#fdf8e4" : "#f2f3f8",
                      color: u.role === "owner" ? "#a07f00" : "#4a4f63",
                    }}>{ROLE_AZ[u.role] ?? u.role}</span>
                  )}
                </td>
                <td>
                  <span style={{ color: u.active ? OK : RED, fontWeight: 600, fontSize: ".85rem" }}>
                    {u.active ? "Aktiv" : "Deaktiv"}
                  </span>
                </td>
                <td style={{ color: GRAY }}>{dt(u.last_login_at)}</td>
                <td style={{ color: GRAY }}>{new Date(u.created_at).toLocaleDateString("az")}</td>
                <td>
                  {canTouch(u) && (
                    <div className="icon-actions">
                      <button className="ibtn" title={u.active ? "Deaktiv et (girişi bağlanır)" : "Aktivləşdir"}
                        disabled={setActive.isPending}
                        onClick={() => setActive.mutate({ id: u.id, active: !u.active })}>
                        {u.active ? "⏸" : "▶"}
                      </button>
                      <button className="ibtn warn" title="Sil"
                        onClick={() => { if (confirm(`${u.email} silinsin?`)) remove.mutate(u.id); }}>
                        {Ic.trash}
                      </button>
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {show && (
        <Modal title="Yeni istifadəçi" onClose={() => setShow(false)}>
          <label>E-mail</label>
          <input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="ad@sirket.az" />
          <label>Müvəqqəti şifrə (min 8 simvol)</label>
          <PasswordInput value={password} onChange={setPassword} placeholder="••••••••" />
          <label>Rol</label>
          <div className="field">
            {(isOwner ? ["admin", "member"] : ["member"]).map((rr) => (
              <div key={rr} onClick={() => setRole(rr)}
                style={{
                  border: `1px solid ${role === rr ? "#f0c000" : "#dfe2ea"}`,
                  background: role === rr ? "#fdf8e4" : "#fff",
                  padding: ".55rem .8rem", marginBottom: ".4rem", cursor: "pointer",
                }}>
                <strong style={{ display: "block", fontSize: ".9rem" }}>{ROLE_AZ[rr]}</strong>
                <span style={{ fontSize: ".8rem", color: GRAY }}>{ROLE_DESC[rr]}</span>
              </div>
            ))}
          </div>
          {error && <p className="error">{error}</p>}
          <div className="modal-actions">
            <button onClick={() => setShow(false)}>Ləğv et</button>
            <button className="btn-primary" disabled={create.isPending || !email.trim() || password.length < 8}
              onClick={() => create.mutate()}>
              {Ic.check} {create.isPending ? "Yaradılır..." : "Yarat"}
            </button>
          </div>
        </Modal>
      )}
    </>
  );
}
