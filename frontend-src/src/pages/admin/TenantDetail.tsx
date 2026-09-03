import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";
import PasswordInput from "../../components/PasswordInput";
import type { Tenant } from "./TenantList";
import { StatusSwitch } from "./TenantList";
import DeleteTenantModal from "./DeleteTenantModal";

interface TUser { id: string; email: string; role: string; created_at: string }
const roleAz: Record<string, string> = { owner: "Sahib", admin: "Admin", member: "Üzv" };

function UserEditRow({ user, tenantId }: { user: TUser; tenantId: string }) {
  const qc = useQueryClient();
  const [email, setEmail] = useState(user.email);
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  useEffect(() => { setEmail(user.email); }, [user.email]);

  const dirty = email !== user.email || password !== "";

  const save = useMutation({
    mutationFn: () => {
      const body: Record<string, string> = {};
      if (email !== user.email) body.email = email;
      if (password) body.password = password;
      return api.patch(`/admin/users/${user.id}`, body);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-tenant-users", tenantId] });
      setPassword(""); setError(""); setSaved(true);
      setTimeout(() => setSaved(false), 2500);
    },
    onError: (e: any) =>
      setError(e.response?.data?.error === "email already registered"
        ? "Bu e-mail artıq istifadə olunur"
        : e.response?.data?.error ?? "Xəta baş verdi"),
  });

  return (
    <div className="uform">
      <div className="uform-grid">
        <div className="field">
          <label>E-mail</label>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div className="field">
          <label>Yeni şifrə <em>(boş = dəyişmir)</em></label>
          <PasswordInput value={password} onChange={setPassword} placeholder="••••••••" />
        </div>
        <div className="field role-field">
          <label>Rol</label>
          <span className={`badge ${user.role === "owner" ? "active" : ""}`}>{roleAz[user.role] ?? user.role}</span>
        </div>
        <div className="field save-field">
          <label>&nbsp;</label>
          <button className="btn-primary sm" disabled={!dirty || save.isPending} onClick={() => save.mutate()}>
            {Ic.check} {save.isPending ? "Saxlanılır..." : saved ? "Yadda saxlandı" : "Yadda saxla"}
          </button>
        </div>
      </div>
      {error && <p className="error">{error}</p>}
      {password && <p className="hint">Şifrə dəyişəndə istifadəçinin köhnə sessiyaları bağlanacaq.</p>}
    </div>
  );
}

export default function TenantDetail() {
  const { id } = useParams();
  const qc = useQueryClient();
  const [showDel, setShowDel] = useState(false);

  const tenants = useQuery({
    queryKey: ["admin-tenants"],
    queryFn: async () => (await api.get<{ tenants: Tenant[] }>("/admin/tenants")).data.tenants,
  });
  const tenant = tenants.data?.find((t) => t.id === id);

  const users = useQuery({
    queryKey: ["admin-tenant-users", id],
    queryFn: async () => (await api.get<{ users: TUser[] }>(`/admin/tenants/${id}/users`)).data.users,
    enabled: !!id,
  });

  const toggle = useMutation({
    mutationFn: () =>
      api.patch(`/admin/tenants/${id}`, { status: tenant?.status === "active" ? "suspended" : "active" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-tenants"] }),
  });

  if (tenants.isLoading) return <p>Yüklənir...</p>;
  if (!tenant) return <><Link to="/admin" className="mini-link back">← Tenantlar</Link><p>Tenant tapılmadı.</p></>;

  return (
    <>
      <Link to="/admin" className="mini-link back">← Tenantlar</Link>
      <div className="page-head">
        <div>
          <h1>{tenant.name}</h1>
          <div className="stat-row">
            <span className="stat-chip mono">{tenant.db_name}</span>
            <span className="stat-chip">{Ic.clock} {new Date(tenant.created_at).toLocaleDateString("az")}</span>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
          <StatusSwitch on={tenant.status === "active"} busy={toggle.isPending} onToggle={() => toggle.mutate()} />
          <button className="btn-delete sm" onClick={() => setShowDel(true)}>{Ic.trash} Sil</button>
        </div>
      </div>

      <div className="panel" style={{ marginTop: "1.2rem" }}>
        <div className="panel-head"><h2>{Ic.users} İstifadəçilər</h2></div>
        <div className="uform-list">
          {users.isLoading ? <div className="expand-pad">Yüklənir...</div> :
            users.data?.map((u) => <UserEditRow key={u.id} user={u} tenantId={id!} />)}
        </div>
      </div>

      {showDel && (
        <DeleteTenantModal tenant={tenant} onClose={() => setShowDel(false)}
          onDeleted={() => window.location.assign("/admin")} />
      )}
    </>
  );
}
