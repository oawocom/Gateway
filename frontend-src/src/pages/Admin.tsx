import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../lib/api";
import { useAuth } from "../store/auth";
import { Ic } from "../components/Icons";
import PasswordInput from "../components/PasswordInput";

interface Tenant {
  id: string; slug: string; name: string; db_name: string;
  status: string; created_at: string; user_count: number;
}
interface TUser { id: string; email: string; role: string; created_at: string }

const roleAz: Record<string, string> = { owner: "Sahib", admin: "Admin", member: "Üzv" };

const chev = (open: boolean) => (
  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2"
    style={{ transform: open ? "rotate(90deg)" : "none", transition: "transform .15s" }}>
    <polyline points="9 6 15 12 9 18" />
  </svg>
);

function TenantUsers({ tenant }: { tenant: Tenant }) {
  const qc = useQueryClient();
  const [editId, setEditId] = useState<string | null>(null);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["admin-tenant-users", tenant.id],
    queryFn: async () =>
      (await api.get<{ users: TUser[] }>(`/admin/tenants/${tenant.id}/users`)).data.users,
  });

  const save = useMutation({
    mutationFn: (u: TUser) => {
      const body: Record<string, string> = {};
      if (email !== u.email) body.email = email;
      if (password) body.password = password;
      return api.patch(`/admin/users/${u.id}`, body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-tenant-users", tenant.id] });
      setEditId(null); setError("");
    },
    onError: (e: any) =>
      setError(e.response?.data?.error === "email already registered"
        ? "Bu e-mail artıq istifadə olunur"
        : e.response?.data?.error ?? "Xəta baş verdi"),
  });

  if (isLoading) return <div className="expand-pad">Yüklənir...</div>;

  return (
    <div className="user-panel">
      {data?.map((u) => (
        <div key={u.id} className="user-row">
          {editId === u.id ? (
            <>
              <input className="inline-input" type="email" value={email}
                onChange={(e) => setEmail(e.target.value)} placeholder="E-mail" autoFocus />
              <PasswordInput className="inline-input" value={password} onChange={setPassword} placeholder="Yeni şifrə (boş = dəyişmir)" />
              <span className={`badge ${u.role === "owner" ? "active" : ""}`}>{roleAz[u.role] ?? u.role}</span>
              <div className="icon-actions">
                <button className="ibtn ok" title="Yadda saxla"
                  disabled={save.isPending || (!password && email === u.email)}
                  onClick={() => save.mutate(u)}>{Ic.check}</button>
                <button className="ibtn" title="Ləğv et" onClick={() => { setEditId(null); setError(""); }}>{Ic.x}</button>
              </div>
            </>
          ) : (
            <>
              <span className="u-email">{u.email}</span>
              <span className="u-date">{new Date(u.created_at).toLocaleDateString("az")}</span>
              <span className={`badge ${u.role === "owner" ? "active" : ""}`}>{roleAz[u.role] ?? u.role}</span>
              <div className="icon-actions">
                <button className="ibtn" title="Redaktə et"
                  onClick={() => { setEditId(u.id); setEmail(u.email); setPassword(""); setError(""); }}>{Ic.edit}</button>
              </div>
            </>
          )}
        </div>
      ))}
      {error && <p className="error" style={{ padding: "0 1rem .6rem" }}>{error}</p>}
    </div>
  );
}

export default function Admin() {
  const { user, logout } = useAuth();
  const qc = useQueryClient();
  const [open, setOpen] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["admin-tenants"],
    queryFn: async () => (await api.get<{ tenants: Tenant[] }>("/admin/tenants")).data.tenants,
  });

  const toggle = useMutation({
    mutationFn: (t: Tenant) =>
      api.patch(`/admin/tenants/${t.id}`, { status: t.status === "active" ? "suspended" : "active" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-tenants"] }),
  });

  const totals = {
    tenants: data?.length ?? 0,
    active: data?.filter((t) => t.status === "active").length ?? 0,
    users: data?.reduce((s, t) => s + t.user_count, 0) ?? 0,
  };

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="side-brand">
          <img src="/logo.webp" alt="AzenRob" />
          <div>
            <strong>Gateway</strong>
            <span>Platform Admin</span>
          </div>
        </div>
        <nav>
          <div className="nav-group">
            <span className="nav-label">İdarəetmə</span>
            <a className="active"><i>{Ic.building}</i>Tenantlar</a>
          </div>
        </nav>
        <div className="side-footer">
          <div className="tenant-chip"><i>{Ic.settings}</i>Superadmin</div>
        </div>
      </aside>
      <div className="content">
        <header className="topbar">
          <div />
          <div className="top-user">
            <div className="avatar">{user?.email?.[0]?.toUpperCase()}</div>
            <span>{user?.email}</span>
            <button className="ibtn" title="Çıxış" onClick={logout}>{Ic.logout}</button>
          </div>
        </header>
        <main>
          <h1>Tenantlar</h1>
          <p className="page-sub">Platformada qeydiyyatdan keçmiş şirkətlər</p>
          <div className="cards" style={{ maxWidth: 640 }}>
            <div className="card stat"><i>{Ic.building}</i><div><span>Tenantlar</span><strong>{totals.tenants}</strong></div></div>
            <div className="card stat"><i>{Ic.activity}</i><div><span>Aktiv</span><strong>{totals.active}</strong></div></div>
            <div className="card stat"><i>{Ic.users}</i><div><span>İstifadəçilər</span><strong>{totals.users}</strong></div></div>
          </div>

          {isLoading ? <p>Yüklənir...</p> : (
            <div className="tenant-list" style={{ marginTop: "1.4rem" }}>
              <div className="tl-head">
                <span></span><span>Şirkət</span><span>Baza</span><span>İstifadəçi</span><span>Status</span><span>Yaradılıb</span><span></span>
              </div>
              {data?.map((t) => (
                <div key={t.id}>
                  <div className="tl-row" onClick={() => setOpen(open === t.id ? null : t.id)}>
                    <span className="tl-chev">{chev(open === t.id)}</span>
                    <span className="tl-name">{t.name}<em>{t.slug}</em></span>
                    <span className="tl-db">{t.db_name}</span>
                    <span>{t.user_count}</span>
                    <span><span className={`badge ${t.status}`}>{t.status === "active" ? "aktiv" : t.status === "suspended" ? "dayandırılıb" : t.status}</span></span>
                    <span>{new Date(t.created_at).toLocaleDateString("az")}</span>
                    <span className="icon-actions" onClick={(e) => e.stopPropagation()}>
                      <button className={`ibtn ${t.status === "active" ? "warn" : "ok"}`}
                        title={t.status === "active" ? "Dayandır" : "Aktivləşdir"}
                        disabled={toggle.isPending} onClick={() => toggle.mutate(t)}>
                        {t.status === "active" ? Ic.pause : Ic.play}
                      </button>
                    </span>
                  </div>
                  {open === t.id && <TenantUsers tenant={t} />}
                </div>
              ))}
            </div>
          )}
        </main>
      </div>
    </div>
  );
}
