import type { ReactNode } from "react";

export default function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="auth-page">
      <div className="brand">
        <img src="/logo.webp" alt="AzenRob" />
        <div>
          <strong>Gateway</strong>
          <span>by AzenRob</span>
        </div>
      </div>
      {children}
      <footer>© {new Date().getFullYear()} AzenRob · Bütün hüquqlar qorunur</footer>
    </div>
  );
}
