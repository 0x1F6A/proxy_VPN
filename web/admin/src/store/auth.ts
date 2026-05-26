import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface AuthState {
  token: string | null
  email: string | null
  role: string | null
  setAuth: (t: string, email: string, role: string) => void
  logout: () => void
}

export const useAuth = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      email: null,
      role: null,
      setAuth: (token, email, role) => set({ token, email, role }),
      logout: () => set({ token: null, email: null, role: null }),
    }),
    { name: 'proxy-vpn-admin-auth' },
  ),
)
