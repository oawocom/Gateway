import { NavLink, Outlet } from "react-router-dom";
import { useAuth } from "../store/auth";
import { Ic } from "../components/Icons";

const groups = [
  {
    label: "Ümumi",
    items: [{ to: "/", label: "İdarə paneli", icon: Ic.dashboard, end: true }],
  },
  {
    label: "Connectorlar",
    items: [
      { to: "/integrations", label: "Marketplace", icon: Ic.store },
      { to: "/connections", label: "Bağlantılarım", icon: Ic.plug },
    ],
  },
  {
    label: "Data",
    items: [
      { to: "/data", label: "Data mərkəzi", icon: Ic.db },
      { to: "/automations", label: "Avtomatlaşdırma", icon: Ic.zap },
    ],
  },
  {
    label: "İdarəetmə",
    items: [
      { to: "/users", label: "İstifadəçilər", icon: Ic.users, roles: ["owner", "admin"] },
      { to: "/settings", label: "Parametrlər", icon: Ic.settings, roles: ["owner", "admin"] },
    ],
  },
];

export default function AppLayout() {
  const { user, tenant, logout } = useAuth();
  return (
    <div className="app">
      <aside className="sidebar">
        <div className="side-brand">
          <img src="/logo.webp" alt="AzenRob" />
          <div>
            <strong>Gateway</strong>
            <span>by AzenRob</span>
          </div>
        </div>
        <nav>
          {groups.map((g) => {
            const items = g.items.filter((m: any) => !m.roles || m.roles.includes(user?.role ?? ""));
            if (!items.length) return null;
            return (
              <div key={g.label} className="nav-group">
                <span className="nav-label">{g.label}</span>
                {items.map((m: any) => (
                  <NavLink key={m.to} to={m.to} end={m.end}
                    className={({ isActive }) => (isActive ? "active" : "")}>
                    <i>{m.icon}</i>{m.label}
                  </NavLink>
                ))}
              </div>
            );
          })}
        </nav>
        <div className="side-footer">
          <div className="tenant-chip"><i>{Ic.building}</i>{tenant?.name}</div>
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
