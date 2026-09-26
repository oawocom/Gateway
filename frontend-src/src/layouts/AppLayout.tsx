import { useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "../store/auth";
import { api } from "../lib/api";
import { Ic } from "../components/Icons";

interface ReportDef { id: string; title: string; group?: string; path: string }
interface C1MenuItem { connection_id: string; connection: string; adapter: string; config_name: string; reports: ReportDef[] }

const groups = [
  {
    label: "Ümumi",
    items: [{ to: "/", label: "İdarə paneli", icon: Ic.dashboard, end: true }],
  },
  {
    label: "Connectorlar",
    items: [
      { to: "/integrations", label: "İnteqrasiyalar", icon: Ic.store },
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
];

// 1C reports: one accordion per recognized 1C connection, titled by the
// name the customer gave the connection; entries come from the adapter
// registry, links are scoped to the connection via ?connection=.
function C1Nav() {
  const loc = useLocation();
  const { data } = useQuery({
    queryKey: ["c1-menu"],
    queryFn: async () => (await api.get<{ items: C1MenuItem[] }>("/reports/c1/menu")).data.items,
    staleTime: 5 * 60 * 1000,
    retry: false,
  });
  const [open, setOpen] = useState<Record<string, boolean>>(() => {
    try { return JSON.parse(localStorage.getItem("c1nav-open") ?? "{}"); } catch { return {}; }
  });
  if (!data?.length) return null;
  const toggle = (id: string) => {
    const next = { ...open, [id]: !(open[id] ?? true) };
    setOpen(next);
    localStorage.setItem("c1nav-open", JSON.stringify(next));
  };
  const curConn = new URLSearchParams(loc.search).get("connection") ?? "";
  const single = data.length === 1;

  return (
    <div className="nav-group">
      <span className="nav-label">1C Hesabatlar</span>
      {data.map((item) => {
        const isOpen = open[item.connection_id] ?? true;
        const groups = [...new Set(item.reports.filter((r) => r.group).map((r) => r.group!))];
        const linkOf = (r: ReportDef) => `${r.path}?connection=${item.connection_id}`;
        const isActive = (r: ReportDef) =>
          loc.pathname === r.path && (single || curConn === item.connection_id || curConn === "");
        const renderLink = (r: ReportDef, indent = false) => (
          <NavLink key={r.id} to={linkOf(r)}
            className={isActive(r) ? "active" : ""}
            style={indent ? { paddingLeft: "2.4rem" } : undefined}>
            <i>{r.group ? Ic.activity : Ic.building}</i>{r.title}
          </NavLink>
        );
        return (
          <div key={item.connection_id}>
            <a onClick={() => toggle(item.connection_id)}
              style={{ cursor: "pointer", display: "flex", alignItems: "center", fontWeight: 700 }}>
              <i style={{ transform: isOpen ? "rotate(90deg)" : "none", transition: "transform .15s", display: "inline-flex" }}>▸</i>
              {item.connection}
            </a>
            {isOpen && (
              <div>
                {item.reports.filter((r) => !r.group).map((r) => renderLink(r, true))}
                {groups.map((g) => (
                  <div key={g}>
                    <span className="nav-label" style={{ paddingLeft: "2.4rem", opacity: 0.8 }}>{g}</span>
                    {item.reports.filter((r) => r.group === g).map((r) => renderLink(r, true))}
                  </div>
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

function UserMenu() {
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const nav = useNavigate();
  const canManage = user?.role === "owner" || user?.role === "admin";
  const go = (to: string) => { setOpen(false); nav(to); };
  return (
    <div style={{ position: "relative" }}>
      <button className="user-chip" onClick={() => setOpen(!open)}>
        <span className="avatar">{user?.email?.[0]?.toUpperCase()}</span>
        <span>{user?.email}</span>
        <i style={{ transform: open ? "rotate(180deg)" : "none", transition: "transform .15s", fontSize: ".7rem" }}>▾</i>
      </button>
      {open && (
        <>
          <div style={{ position: "fixed", inset: 0, zIndex: 40 }} onClick={() => setOpen(false)} />
          <div className="user-menu">
            <div className="user-menu-head">
              <strong>{user?.email}</strong>
              <span>{user?.role === "owner" ? "Sahib" : user?.role === "admin" ? "Admin" : "İstifadəçi"}</span>
            </div>
            {canManage && (
              <>
                <a onClick={() => go("/users")}><i>{Ic.users}</i>İstifadəçilər</a>
                <a onClick={() => go("/settings")}><i>{Ic.settings}</i>Parametrlər</a>
              </>
            )}
          </div>
        </>
      )}
    </div>
  );
}

export default function AppLayout() {
  const { user, tenant, logout } = useAuth();
  const qc = useQueryClient();
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
          <C1Nav />
        </nav>
        <div className="side-footer" style={{ display: "flex", alignItems: "center", gap: ".5rem" }}>
          <div className="tenant-chip" style={{ flex: 1, justifyContent: "flex-start", textAlign: "left" }}>
            <i>{Ic.building}</i>{tenant?.name}
          </div>
          <button className="ibtn" title="Çıxış" onClick={() => { qc.clear(); logout(); }}>{Ic.logout}</button>
        </div>
      </aside>
      <div className="content">
        <header className="topbar">
          <div />
          <UserMenu />
        </header>
        <main><Outlet /></main>
      </div>
    </div>
  );
}
