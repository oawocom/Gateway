import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";
import DeleteTenantModal from "./DeleteTenantModal";

export interface Tenant {
  id: string; slug: string; name: string; db_name: string;
  status: string; created_at: string; user_count: number;
}

export function StatusSwitch({ on, onToggle, busy }: {
  on: boolean; onToggle: () => void; busy?: boolean;
}) {
  return (
    <span className="switch-wrap" onClick={(e) => e.stopPropagation()}>
      <button type="button" className={`switch ${on ? "on" : ""}`} disabled={busy}
        title={on ? "Deaktiv et" : "Aktivləşdir"} onClick={onToggle} />
      <em className={on ? "sw-on" : "sw-off"}>{on ? "aktiv" : "deaktiv"}</em>
    </span>
  );
}

export default function TenantList() {
  const nav = useNavigate();
  const qc = useQueryClient();
  const [q, setQ] = useState("");
  const [delTenant, setDelTenant] = useState<Tenant | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["admin-tenants"],
    queryFn: async () => (await api.get<{ tenants: Tenant[] }>("/admin/tenants")).data.tenants,
  });

  const toggle = useMutation({
    mutationFn: (t: Tenant) =>
      api.patch(`/admin/tenants/${t.id}`, { status: t.status === "active" ? "suspended" : "active" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin-tenants"] }),
  });

  const shown = data?.filter((t) =>
    !q || t.name.toLowerCase().includes(q.toLowerCase()) || t.slug.includes(q.toLowerCase()));

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Tenantlar</h1>
          <p className="page-sub">Platformada qeydiyyatdan keçmiş şirkətlər</p>
        </div>
        <div className="head-tools">
          <div className="search-box">
            {Ic.search}
            <input placeholder="Axtar..." value={q} onChange={(e) => setQ(e.target.value)} />
          </div>
          <Link className="btn-primary" to="/admin/tenants/new">{Ic.plus} Əlavə et</Link>
        </div>
      </div>

      {isLoading ? <p>Yüklənir...</p> : (
        <div className="tenant-list" style={{ marginTop: "1.1rem" }}>
          <div className="tl-head">
            <span>Şirkət</span><span>Baza</span><span>İstifadəçi</span><span>Yaradılıb</span><span>Status</span><span className="ta-r">Əməliyyatlar</span>
          </div>
          {shown?.map((t) => (
            <div key={t.id} className="tl-row" onClick={() => nav(`/admin/tenants/${t.id}`)}>
              <span className="tl-name">{t.name}<em>{t.slug}</em></span>
              <span className="tl-db">{t.db_name}</span>
              <span>{t.user_count}</span>
              <span>{new Date(t.created_at).toLocaleDateString("az")}</span>
              <StatusSwitch on={t.status === "active"} busy={toggle.isPending} onToggle={() => toggle.mutate(t)} />
              <span className="icon-actions" onClick={(e) => e.stopPropagation()}>
                <button className="ibtn" title="Redaktə et" onClick={() => nav(`/admin/tenants/${t.id}`)}>{Ic.edit}</button>
                <button className="ibtn warn" title="Sil" onClick={() => setDelTenant(t)}>{Ic.trash}</button>
              </span>
            </div>
          ))}
          {!shown?.length && <div className="expand-pad">Nəticə tapılmadı.</div>}
        </div>
      )}

      {delTenant && (
        <DeleteTenantModal tenant={delTenant} onClose={() => setDelTenant(null)} />
      )}
    </>
  );
}
