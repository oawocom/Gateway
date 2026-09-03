import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../store/auth";
import AuthLayout from "../components/AuthLayout";
import PasswordInput from "../components/PasswordInput";

export default function Register() {
  const nav = useNavigate();
  const setSession = useAuth((s) => s.setSession);
  const [company, setCompany] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    if (password !== confirm) {
      setError("Şifrələr uyğun gəlmir");
      return;
    }
    setLoading(true);
    try {
      const { data } = await api.post("/auth/register", { company, email, password });
      setSession(data);
      nav("/");
    } catch (err: any) {
      setError(err.response?.data?.error === "email already registered"
        ? "Bu e-mail artıq qeydiyyatdan keçib"
        : err.response?.data?.error ?? "Qeydiyyat mümkün olmadı");
    } finally {
      setLoading(false);
    }
  }

  return (
    <AuthLayout>
      <form className="auth-card" onSubmit={submit}>
        <h1>Qeydiyyat</h1>
        <p className="sub">Şirkətiniz üçün yeni iş sahəsi yaradın</p>
        <label>Şirkət adı</label>
        <input placeholder="Şirkətinizin adı" value={company} onChange={(e) => setCompany(e.target.value)} required />
        <label>E-mail</label>
        <input type="email" placeholder="ad@sirket.com" value={email} onChange={(e) => setEmail(e.target.value)} required />
        <label>Şifrə</label>
        <PasswordInput placeholder="Minimum 8 simvol" value={password} onChange={setPassword} minLength={8} required />
        <label>Şifrənin təsdiqi</label>
        <PasswordInput placeholder="Şifrəni təkrar daxil edin" value={confirm} onChange={setConfirm} minLength={8} required />
        {error && <p className="error">{error}</p>}
        <button disabled={loading}>{loading ? "Yaradılır..." : "Qeydiyyatdan keç"}</button>
        <p className="alt">Hesabınız var? <Link to="/login">Daxil olun</Link></p>
      </form>
    </AuthLayout>
  );
}
