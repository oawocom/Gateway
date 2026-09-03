import { Navigate, Outlet } from "react-router-dom";
import { useAuth } from "../store/auth";

export function ProtectedRoute() {
  const token = useAuth((s) => s.accessToken);
  return token ? <Outlet /> : <Navigate to="/login" replace />;
}

export function AdminRoute() {
  const { accessToken, user } = useAuth();
  if (!accessToken) return <Navigate to="/login" replace />;
  return user?.role === "superadmin" ? <Outlet /> : <Navigate to="/" replace />;
}

export function HomeRedirect() {
  const user = useAuth((s) => s.user);
  return user?.role === "superadmin" ? <Navigate to="/admin" replace /> : <Outlet />;
}
