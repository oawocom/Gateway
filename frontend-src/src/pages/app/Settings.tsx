import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { useAuth } from "../../store/auth";
import DataTable, { type Col } from "../../components/DataTable";
import { Ic } from "../../components/Icons";

interface LogRow {
  id: number; connection_name: string; entity_name: string;
  status: string; message: string; records_synced: number; created_at: string;
}
interface Rules { default_due_days: number; settle: "customer" | "contract" }

const TABS = ["Ümumi", "Hesabat qaydaları", "Jurnal"] as const;

export default function Settings() {
  const { tenant, user, renameTenant } = useAuth();
  const qc = useQueryClient();
  const [tab, setTab] = useState<(typeof TABS)[number]>("Ümumi");
  const canEdit = user?.role === "owner" || user?.role === "admin";

  return (
    <>
      <div className="page-head">
        <div>
          <h1>Parametrlər</h1>
          <p className="page-sub">İş sahəsi, hesabat qaydaları və sinxronizasiya jurnalı</p>
        </div>
      </div>

      <div className="tabs">
        {TABS.map((t) => (
          <button key={t} className={tab === t ? "tab active" : "tab"} onClick={() => setTab(t)}>{t}</button>
        ))}
      </div>

      {tab === "Ümumi" && <General tenant={tenant} user={user} renameTenant={renameTenant} />}
      {tab === "Hesabat qaydaları" && <ReportRules canEdit={canEdit} qc={qc} />}
      {tab === "Jurnal" && <Journal />}
    </>
  );
}

function General({ tenant, user, renameTenant }: any) {
  const [name, setName] = useState<string | null>(null);
  const [msg, setMsg] = useState("");
  const val = name ?? tenant?.name ?? "";
  const isOwner = user?.role === "owner";
  const save = useMutation({
    mutationFn: () => api.put("/tenant/name", { name: val }),
    onSuccess: () => { renameTenant(val.trim()); setMsg("✓ Yadda saxlandı"); setTimeout(() => setMsg(""), 2000); },
    onError: (e: any) => setMsg("✗ " + (e.response?.data?.error ?? "Xəta")),
  });
  const roleAz = user?.role === "owner" ? "Sahib" : user?.role === "admin" ? "Admin" : "Analitik";

  return (
    <div style={{ maxWidth: 560 }}>
      <div className="panel">
        <h3>Şirkət profili</h3>
        <label>Şirkət adı (sol menyuda görünür)</label>
        <div style={{ display: "flex", gap: ".5rem" }}>
          <input value={val} onChange={(e) => setName(e.target.value)} disabled={!isOwner} style={{ flex: 1 }} />
          {isOwner && (
            <button className="btn-primary" disabled={save.isPending || val.trim() === tenant?.name}
              onClick={() => save.mutate()}>{Ic.check} Saxla</button>
          )}
        </div>
        {!isOwner && <p className="page-sub" style={{ marginTop: ".4rem" }}>Adı yalnız sahib dəyişə bilər.</p>}
        {msg && <p style={{ marginTop: ".4rem", fontSize: ".85rem" }}>{msg}</p>}
      </div>
      <div className="cards" style={{ marginTop: "1rem" }}>
        <div className="card"><span>Slug</span><strong style={{ fontSize: "1.05rem" }}>{tenant?.slug}</strong></div>
        <div className="card"><span>Rolunuz</span><strong style={{ fontSize: "1.05rem" }}>{roleAz}</strong></div>
      </div>
    </div>
  );
}

function ReportRules({ canEdit, qc }: any) {
  const [form, setForm] = useState<Rules | null>(null);
  const [msg, setMsg] = useState("");
  const cur = useQuery({
    queryKey: ["settings"],
    queryFn: async () => (await api.get<Record<string, any>>("/settings")).data,
  });
  const rules: Rules = form ?? cur.data?.report_rules ?? { default_due_days: 14, settle: "customer" };
  const save = useMutation({
    mutationFn: () => api.put("/settings/report_rules", rules),
    onSuccess: () => {
      setMsg("✓ Yadda saxlandı — hesabatlar yeni qaydalarla hesablanacaq");
      qc.invalidateQueries({ predicate: (q: any) => String(q.queryKey[0]).startsWith("c1") });
      setTimeout(() => setMsg(""), 2500);
    },
    onError: (e: any) => setMsg("✗ " + (e.response?.data?.error ?? "Xəta")),
  });

  return (
    <div style={{ maxWidth: 560 }}>
      <div className="panel">
        <h3>Debitor hesablanması</h3>
        <label>Standart ödəniş şərti (gün)</label>
        <p className="page-sub" style={{ margin: "0 0 .35rem" }}>
          İnvoysda ödəniş tarixi göstərilməyibsə, borc invoice tarixindən bu qədər gün sonra gecikmiş sayılır.
        </p>
        <input type="number" min={0} max={365} value={rules.default_due_days} disabled={!canEdit}
          onChange={(e) => setForm({ ...rules, default_due_days: +e.target.value })} style={{ width: 120 }} />

        <label style={{ marginTop: "1rem" }}>Ödənişlərin bağlanma rejimi</label>
        <p className="page-sub" style={{ margin: "0 0 .35rem" }}>
          Ödənişlər invoyslara hansı səviyyədə uyğunlaşdırılsın.
        </p>
        <select value={rules.settle} disabled={!canEdit}
          onChange={(e) => setForm({ ...rules, settle: e.target.value as Rules["settle"] })}>
          <option value="customer">Müştəri üzrə (standart)</option>
          <option value="contract">Müqavilə üzrə</option>
        </select>

        {canEdit && (
          <div style={{ marginTop: "1rem" }}>
            <button className="btn-primary" disabled={save.isPending || !form} onClick={() => save.mutate()}>
              {Ic.check} {save.isPending ? "Yadda saxlanılır..." : "Yadda saxla"}
            </button>
          </div>
        )}
        {msg && <p style={{ marginTop: ".5rem", fontSize: ".85rem" }}>{msg}</p>}
      </div>
    </div>
  );
}

function Journal() {
  const { data, isLoading } = useQuery({
    queryKey: ["synclog-full"],
    queryFn: async () => (await api.get<{ log: LogRow[] }>("/data/synclog")).data.log,
  });
  const cols: Col<LogRow>[] = [
    { key: "created_at", label: "Vaxt", val: (r) => r.created_at, render: (r) => new Date(r.created_at).toLocaleString("az") },
    { key: "connection_name", label: "Bağlantı", val: (r) => r.connection_name },
    { key: "entity_name", label: "Entity", val: (r) => r.entity_name },
    { key: "status", label: "Status", val: (r) => r.status, render: (r) => (
      <span className={`badge ${r.status === "success" ? "active" : "suspended"}`}>{r.status}</span>
    ) },
    { key: "records_synced", label: "Qeyd", val: (r) => r.records_synced, align: "right" },
    { key: "message", label: "Mesaj", val: (r) => r.message },
  ];
  if (isLoading) return <p>Yüklənir...</p>;
  return <DataTable rows={data ?? []} cols={cols} exportName="jurnal" searchHint="Bağlantı, entity, status..." />;
}
