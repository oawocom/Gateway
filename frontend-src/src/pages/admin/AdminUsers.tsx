import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import { Ic } from "../../components/Icons";
import PasswordInput from "../../components/PasswordInput";

interface Admin { id: string; email: string; created_at: string }

function AdminRow({ admin, canDelete }: { admin: Admin; canDelete: boolean }) {
  const qc = useQueryClient();
  const me = useAuth((s) => s.user);
  const [email, setEmail] = useState(admin.email);
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  useEffect(() => { setEmail(admin.email); }, [admin.email]);

  const dirty = email !== admin.email || password !== "";
  const isSelf = me?.id === admin.id;

  const save = useMutation({
    mutationFn: () => {
      const body: Record<string, string> = {};
      if (email !== admin.email) body.email = email;
      if (password) body.password = password;
      return api.patch(`/admin/admins/${admin.id}`, body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-admins"] });
      setPassword(""); setError(""); setSaved(true);
      setTimeout(() => setSaved(false), 2500);
    },
    onError: (e: any) =>
      setError(e.response?.data?.error === "email already registered"
        ? "Bu e-mail artıq istifadə olunur"
        : e.response?.data?.error ?? "Xəta baş verdi"),
  });

  const del = useMutation({
    mutationFn: () => api.delete(`/admin/admins/${admin.id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-admins"] }),
    onError: (e: any) => setError(e.response?.data?.error ?? "Silinə bilmədi"),
  });

  return (
    <div className="uform">
      <div className="uform-grid admins">
        <div className="field">
          <label>E-mail {isSelf && <em>(siz)</em>}</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div className="field">
          <label>Yeni şifrə <em>(boş = dəyişmir)</em></label>
          <PasswordInput value={password} onChange={setPassword} placeholder="••••••••" />
        </div>
        <div className="field save-field">
          <label>&nbsp;</label>
          <div style={{ display: "flex", gap: ".5rem" }}>
            <button className="btn-primary sm" disabled={!dirty || save.isPending} onClick={() => save.mutate()}>
              {Ic.check} {save.isPending ? "Saxlanılır..." : saved ? "Yadda saxlandı" : "Yadda saxla"}
            </button>
            {!isSelf && canDelete && (
              <button className="ibtn warn" title="Sil"
                onClick={() => { if (confirm(`${admin.email} silinsin?`)) del.mutate(); }}>{Ic.trash}</button>
            )}
          </div>
        </div>
      </div>
      {error && <p className="error">{error}</p>}
    </div>
  );
}

export default function AdminUsers() {
  const qc = useQueryClient();
  const [show, setShow] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["admin-admins"],
    queryFn: async () => (await api.get<{ admins: Admin[] }>("/admin/admins")).data.admins,
  });

  const create = useMutation({
    mutationFn: () => api.post("/admin/admins", { email, password }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-admins"] });
      setShow(false); setEmail(""); setPassword(""); setError("");
    },
    onError: (e: any) =>
      setError(e.response?.data?.error === "email already registered"
        ? "Bu e-mail artıq qeydiyyatdan keçib"
        : e.response?.data?.error ?? "Xəta baş verdi"),
  });

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Adminlər</h1>
          <p className="page-sub">Platformanı idarə edən superadmin hesabları</p>
        </div>
        <button className="btn-primary" onClick={() => { setShow(!show); setError(""); }}>{Ic.plus} Əlavə et</button>
      </div>

      {show && (
        <div className="panel form-wide" style={{ marginTop: "1.1rem" }}>
          <div className="uform-grid admins">
            <div className="field">
              <label>E-mail</label>
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="admin2@oawo.com" />
            </div>
            <div className="field">
              <label>Şifrə</label>
              <PasswordInput value={password} onChange={setPassword} placeholder="Minimum 8 simvol" />
            </div>
            <div className="field save-field">
              <label>&nbsp;</label>
              <div style={{ display: "flex", gap: ".5rem" }}>
                <button className="btn-primary sm" disabled={create.isPending || !email || password.length < 8}
                  onClick={() => create.mutate()}>{Ic.check} Yarat</button>
                <button className="ibtn" title="Bağla" onClick={() => setShow(false)}>{Ic.x}</button>
              </div>
            </div>
          </div>
          {error && <p className="error">{error}</p>}
        </div>
      )}

      <div className="panel" style={{ marginTop: "1.1rem" }}>
        <div className="uform-list">
          {isLoading ? <div className="expand-pad">Yüklənir...</div> :
            data?.map((a) => <AdminRow key={a.id} admin={a} canDelete={(data?.length ?? 0) > 1} />)}
        </div>
      </div>
    </>
  );
}
