import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../store/auth";
import AuthLayout from "../components/AuthLayout";
import PasswordInput from "../components/PasswordInput";

export default function Login() {
  const nav = useNavigate();
  const setSession = useAuth((s) => s.setSession);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const { data } = await api.post("/auth/login", { email, password });
      setSession(data);
      nav(data.user?.role === "superadmin" ? "/admin" : "/");
    } catch (err: any) {
      setError(err.response?.data?.error === "invalid credentials"
        ? "E-mail və ya şifrə yanlışdır"
        : err.response?.data?.error ?? "Daxil olmaq mümkün olmadı");
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthLayout>
      <form className="auth-card" onSubmit={submit}>
        <h1>Daxil olun</h1>
        <p className="sub">Hesabınıza daxil olmaq üçün məlumatlarınızı daxil edin</p>
        <label>E-mail</label>
        <input type="email" placeholder="ad@sirket.com" value={email} onChange={(e) => setEmail(e.target.value)} required />
        <label>Şifrə</label>
        <PasswordInput placeholder="••••••••" value={password} onChange={setPassword} required />
        {error && <p className="error">{error}</p>}
        <button disabled={loading}>{loading ? "Daxil olunur..." : "Daxil ol"}</button>
        <p className="alt">Hesabınız yoxdur? <Link to="/register">Qeydiyyat</Link></p>
      </form>
    </AuthLayout>
  );
}
