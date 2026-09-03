import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface User { id: string; email: string; role: string }
export interface Tenant { id: string; name: string; slug: string }

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  user: User | null;
  tenant: Tenant | null;
  setTokens: (access: string, refresh: string) => void;
  setSession: (data: { access_token: string; refresh_token: string; user: User; tenant?: Tenant }) => void;
  logout: () => void;
}

export const useAuth = create<AuthState>()(
  persist(
    (set) => ({
      accessToken: null,
      refreshToken: null,
      user: null,
      tenant: null,
      setTokens: (accessToken, refreshToken) => set({ accessToken, refreshToken }),
      setSession: (d) =>
        set({
          accessToken: d.access_token,
          refreshToken: d.refresh_token,
          user: d.user,
          tenant: d.tenant ?? null,
        }),
      logout: () => set({ accessToken: null, refreshToken: null, user: null, tenant: null }),
    }),
    { name: "gw-auth" }
  )
);
