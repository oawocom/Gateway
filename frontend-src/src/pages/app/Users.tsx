import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import Modal from "../../components/Modal";
import PasswordInput from "../../components/PasswordInput";

interface U { id: string; email: string; role: string; created_at: string }

const roleAz: Record<string, string> = { owner: "Sahib", admin: "Admin", member: "Üzv" };

export default function Users() {
  const me = useAuth((s) => s.user);
  const qc = useQueryClient();
  const [show, setShow] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("member");
  const [error, setError] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["users"],
    queryFn: async () => (await api.get<{ users: U[] }>("/users")).data.users,
  });

  const invalidate = () => qc.invalidateQueries({ queryKey: ["users"] });

  const create = useMutation({
    mutationFn: () => api.post("/users", { email, password, role }),
    onSuccess: () => { invalidate(); setShow(false); setEmail(""); setPassword(""); setRole("member"); setError(""); },
    onError: (e: any) => setError(e.response?.data?.error ?? "Xəta baş verdi"),
  });

  const changeRole = useMutation({
    mutationFn: (p: { id: string; role: string }) => api.patch(`/users/${p.id}`, { role: p.role }),
    onSuccess: invalidate,
  });

  const remove = useMutation({
    mutationFn: (id: string) => api.delete(`/users/${id}`),
    onSuccess: invalidate,
  });

  const isOwner = me?.role === "owner";

  return (
    <>
      <div className="page-head">
        <div>
          <h1>İstifadəçilər</h1>
          <p className="page-sub">Komanda üzvlərini idarə edin</p>
        </div>
        <button className="btn-primary" onClick={() => setShow(true)}>+ İstifadəçi əlavə et</button>
      </div>

      {isLoading ? <p>Yüklənir...</p> : (
        <table className="tbl">
          <thead><tr><th>E-mail</th><th>Rol</th><th>Yaradılıb</th><th></th></tr></thead>
          <tbody>
            {data?.map((u) => (
              <tr key={u.id}>
                <td>{u.email}{u.id === me?.id && " (siz)"}</td>
                <td>
                  {u.role === "owner" || u.id === me?.id || (!isOwner && u.role === "admin") ? (
                    <span className={`badge ${u.role === "owner" ? "active" : ""}`}>{roleAz[u.role]}</span>
                  ) : (
                    <select value={u.role} onChange={(e) => changeRole.mutate({ id: u.id, role: e.target.value })}>
                      {isOwner && <option value="admin">Admin</option>}
                      <option value="member">Üzv</option>
                    </select>
                  )}
                </td>
                <td>{new Date(u.created_at).toLocaleDateString("az")}</td>
                <td>
                  {u.role !== "owner" && u.id !== me?.id && (isOwner || u.role !== "admin") && (
                    <button className="btn-danger" onClick={() => { if (confirm(`${u.email} silinsin?`)) remove.mutate(u.id); }}>Sil</button>
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
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="ad@sirket.com" />
          <label>Müvəqqəti şifrə</label>
          <PasswordInput value={password} onChange={setPassword} placeholder="Minimum 8 simvol" />
          <label>Rol</label>
          <select value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="member">Üzv</option>
            {isOwner && <option value="admin">Admin</option>}
          </select>
          {error && <p className="error">{error}</p>}
          <div className="modal-actions">
            <button onClick={() => setShow(false)}>Ləğv et</button>
            <button className="btn-primary" disabled={create.isPending} onClick={() => create.mutate()}>Əlavə et</button>
          </div>
        </Modal>
      )}
    </>
  );
}
