import { Layout as AntLayout, Menu, Button, theme } from 'antd'
import {
  DashboardOutlined,
  UserOutlined,
  ShoppingOutlined,
  PayCircleOutlined,
  AppstoreOutlined,
  GiftOutlined,
  DatabaseOutlined,
  CloudServerOutlined,
  GroupOutlined,
  BarChartOutlined,
  LogoutOutlined,
} from '@ant-design/icons'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../store/auth'

const { Header, Sider, Content } = AntLayout

const items = [
  { key: '/', icon: <DashboardOutlined />, label: <Link to="/">仪表盘</Link> },
  { key: '/users', icon: <UserOutlined />, label: <Link to="/users">用户</Link> },
  { key: '/orders', icon: <ShoppingOutlined />, label: <Link to="/orders">订单</Link> },
  { key: '/payments', icon: <PayCircleOutlined />, label: <Link to="/payments">支付</Link> },
  { key: '/plans', icon: <AppstoreOutlined />, label: <Link to="/plans">套餐</Link> },
  { key: '/coupons', icon: <GiftOutlined />, label: <Link to="/coupons">优惠券</Link> },
  { key: '/data-packs', icon: <DatabaseOutlined />, label: <Link to="/data-packs">流量包</Link> },
  { key: '/nodes', icon: <CloudServerOutlined />, label: <Link to="/nodes">节点</Link> },
  { key: '/node-groups', icon: <GroupOutlined />, label: <Link to="/node-groups">节点分组</Link> },
  { key: '/reports', icon: <BarChartOutlined />, label: <Link to="/reports">报表</Link> },
]

export function Layout() {
  const loc = useLocation()
  const nav = useNavigate()
  const { email, logout } = useAuth()
  const { token } = theme.useToken()
  return (
    <AntLayout style={{ minHeight: '100vh' }}>
      <Sider breakpoint="lg" collapsedWidth={0} theme="dark">
        <div style={{ color: 'white', padding: 16, fontSize: 16, fontWeight: 600 }}>
          proxy_VPN Admin
        </div>
        <Menu theme="dark" selectedKeys={[loc.pathname]} mode="inline" items={items} />
      </Sider>
      <AntLayout>
        <Header
          style={{
            background: token.colorBgContainer,
            display: 'flex',
            justifyContent: 'flex-end',
            alignItems: 'center',
            gap: 12,
            paddingRight: 24,
          }}
        >
          <span style={{ color: token.colorTextSecondary }}>{email}</span>
          <Button
            icon={<LogoutOutlined />}
            onClick={() => {
              logout()
              nav('/login')
            }}
          >
            退出
          </Button>
        </Header>
        <Content style={{ margin: 24 }}>
          <Outlet />
        </Content>
      </AntLayout>
    </AntLayout>
  )
}
