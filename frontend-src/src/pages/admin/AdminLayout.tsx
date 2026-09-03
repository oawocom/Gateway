import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../../store/auth";
import { Ic } from "../../components/Icons";

export default function AdminLayout() {
  const { user, logout } = useAuth();
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
            <NavLink to="/admin" end className={({ isActive }) => (isActive ? "active" : "")}>
              <i>{Ic.building}</i>Tenantlar
            </NavLink>
            <NavLink to="/admin/agents" className={({ isActive }) => (isActive ? "active" : "")}>
              <i>{Ic.bot}</i>AI Agentlər
            </NavLink>
            <NavLink to="/admin/admins" className={({ isActive }) => (isActive ? "active" : "")}>
              <i>{Ic.shield}</i>Adminlər
            </NavLink>
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
        <main><Outlet /></main>
      </div>
    </div>
  );
}
