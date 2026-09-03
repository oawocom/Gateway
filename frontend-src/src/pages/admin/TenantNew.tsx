import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../../lib/api";
import { Ic } from "../../components/Icons";
import PasswordInput from "../../components/PasswordInput";

export default function TenantNew() {
  const nav = useNavigate();
  const qc = useQueryClient();
  const [company, setCompany] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");

  const create = useMutation({
    mutationFn: () => api.post("/admin/tenants", { company, email, password }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-tenants"] });
      nav("/admin");
    },
    onError: (e: any) =>
      setError(e.response?.data?.error === "email already registered"
        ? "Bu e-mail artıq qeydiyyatdan keçib"
        : e.response?.data?.error ?? "Xəta baş verdi"),
  });

  return (
    <>
      <Link to="/admin" className="mini-link back">← Tenantlar</Link>
      <h1>Yeni tenant</h1>
      <p className="page-sub">Yeni şirkət və onun sahib (owner) hesabını yaradın</p>

      <form className="form-page" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <label>Şirkət adı</label>
        <input value={company} onChange={(e) => setCompany(e.target.value)} placeholder="məs. Azersun MMC" required />
        <label>Sahib e-mail</label>
        <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="tenant2@oawo.com" required />
        <label>Sahib şifrəsi</label>
        <PasswordInput value={password} onChange={setPassword} placeholder="Minimum 8 simvol" minLength={8} required />
        {error && <p className="error">{error}</p>}
        <div className="form-actions">
          <Link to="/admin" className="btn-ghost">Ləğv et</Link>
          <button className="btn-primary" disabled={create.isPending || !company || !email || password.length < 8}>
            {Ic.check} {create.isPending ? "Yaradılır..." : "Yarat"}
          </button>
        </div>
      </form>
    </>
  );
}
