import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Login from "./pages/Login";
import Register from "./pages/Register";
import AdminLayout from "./pages/admin/AdminLayout";
import TenantList from "./pages/admin/TenantList";
import TenantNew from "./pages/admin/TenantNew";
import TenantDetail from "./pages/admin/TenantDetail";
import AgentList from "./pages/admin/AgentList";
import AgentForm from "./pages/admin/AgentForm";
import AdminUsers from "./pages/admin/AdminUsers";
import AppLayout from "./layouts/AppLayout";
import Home from "./pages/app/Home";
import Integrations from "./pages/app/Integrations";
import Connections from "./pages/app/Connections";
import Data from "./pages/app/Data";
import Reports from "./pages/app/Reports";
import Dashboard1C from "./pages/app/Dashboard1C";
import Automations from "./pages/app/Automations";
import Users from "./pages/app/Users";
import Settings from "./pages/app/Settings";
import { ProtectedRoute, AdminRoute, HomeRedirect } from "./components/ProtectedRoute";

const qc = new QueryClient({ defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } } });

export default function App() {
  return (
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route element={<ProtectedRoute />}>
            <Route element={<HomeRedirect />}>
              <Route element={<AppLayout />}>
                <Route path="/" element={<Home />} />
                <Route path="/integrations" element={<Integrations />} />
                <Route path="/connections" element={<Connections />} />
                <Route path="/data" element={<Data />} />
                <Route path="/reports" element={<Reports />} />
                <Route path="/reports/1c" element={<Dashboard1C />} />
                <Route path="/automations" element={<Automations />} />
                <Route path="/users" element={<Users />} />
                <Route path="/settings" element={<Settings />} />
              </Route>
            </Route>
          </Route>
          <Route element={<AdminRoute />}>
            <Route element={<AdminLayout />}>
              <Route path="/admin" element={<TenantList />} />
              <Route path="/admin/tenants/new" element={<TenantNew />} />
              <Route path="/admin/tenants/:id" element={<TenantDetail />} />
              <Route path="/admin/agents" element={<AgentList />} />
              <Route path="/admin/agents/new" element={<AgentForm />} />
              <Route path="/admin/agents/:id" element={<AgentForm />} />
              <Route path="/admin/admins" element={<AdminUsers />} />
            </Route>
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
