import { createBrowserRouter, Navigate } from 'react-router-dom'
import { Layout } from './layout/Layout'
import { Login } from './pages/Login'
import { Dashboard } from './pages/Dashboard'
import { Users } from './pages/Users'
import { Orders } from './pages/Orders'
import { Payments } from './pages/Payments'
import { Plans } from './pages/Plans'
import { Coupons } from './pages/Coupons'
import { DataPacks } from './pages/DataPacks'
import { Nodes } from './pages/Nodes'
import { NodeGroups } from './pages/NodeGroups'
import { Reports } from './pages/Reports'
import { useAuth } from './store/auth'

function Protected({ children }: { children: React.ReactNode }) {
  const tok = useAuth((s) => s.token)
  if (!tok) return <Navigate to="/login" replace />
  return <>{children}</>
}

export const router = createBrowserRouter([
  { path: '/login', element: <Login /> },
  {
    path: '/',
    element: (
      <Protected>
        <Layout />
      </Protected>
    ),
    children: [
      { index: true, element: <Dashboard /> },
      { path: 'users', element: <Users /> },
      { path: 'orders', element: <Orders /> },
      { path: 'payments', element: <Payments /> },
      { path: 'plans', element: <Plans /> },
      { path: 'coupons', element: <Coupons /> },
      { path: 'data-packs', element: <DataPacks /> },
      { path: 'nodes', element: <Nodes /> },
      { path: 'node-groups', element: <NodeGroups /> },
      { path: 'reports', element: <Reports /> },
    ],
  },
])
