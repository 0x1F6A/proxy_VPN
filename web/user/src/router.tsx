import { createBrowserRouter, Navigate } from 'react-router-dom'
import { Layout } from './layout/Layout'
import { Login } from './pages/Login'
import { Register } from './pages/Register'
import { Dashboard } from './pages/Dashboard'
import { Plans } from './pages/Plans'
import { Orders } from './pages/Orders'
import { Pay } from './pages/Pay'
import { Subscription } from './pages/Subscription'
import { Nodes } from './pages/Nodes'
import { Invite } from './pages/Invite'
import { Profile } from './pages/Profile'
import { Help } from './pages/Help'
import { useAuth } from './store/auth'

function Protected({ children }: { children: React.ReactNode }) {
  const tok = useAuth((s) => s.token)
  if (!tok) return <Navigate to="/login" replace />
  return <>{children}</>
}

export const router = createBrowserRouter([
  { path: '/login', element: <Login /> },
  { path: '/register', element: <Register /> },
  {
    path: '/',
    element: (
      <Protected>
        <Layout />
      </Protected>
    ),
    children: [
      { index: true, element: <Dashboard /> },
      { path: 'plans', element: <Plans /> },
      { path: 'orders', element: <Orders /> },
      { path: 'pay/:no', element: <Pay /> },
      { path: 'subscription', element: <Subscription /> },
      { path: 'nodes', element: <Nodes /> },
      { path: 'invite', element: <Invite /> },
      { path: 'profile', element: <Profile /> },
      { path: 'help', element: <Help /> },
    ],
  },
])
