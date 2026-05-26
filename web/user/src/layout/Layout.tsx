import { Layout as AntLayout, Menu, Button, theme } from 'antd'
import {
  DashboardOutlined,
  AppstoreOutlined,
  ShoppingOutlined,
  LinkOutlined,
  CloudServerOutlined,
  GiftOutlined,
  UserOutlined,
  QuestionCircleOutlined,
  LogoutOutlined,
} from '@ant-design/icons'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../store/auth'

const { Header, Sider, Content } = AntLayout

const items = [
  { key: '/', icon: <DashboardOutlined />, label: <Link to="/">仪表盘</Link> },
  { key: '/plans', icon: <AppstoreOutlined />, label: <Link to="/plans">套餐</Link> },
  { key: '/orders', icon: <ShoppingOutlined />, label: <Link to="/orders">我的订单</Link> },
  { key: '/subscription', icon: <LinkOutlined />, label: <Link to="/subscription">订阅</Link> },
  { key: '/nodes', icon: <CloudServerOutlined />, label: <Link to="/nodes">节点</Link> },
  { key: '/invite', icon: <GiftOutlined />, label: <Link to="/invite">邀请</Link> },
  { key: '/profile', icon: <UserOutlined />, label: <Link to="/profile">我的账号</Link> },
  { key: '/help', icon: <QuestionCircleOutlined />, label: <Link to="/help">帮助</Link> },
]

export function Layout() {
  const loc = useLocation()
  const nav = useNavigate()
  const { email, logout } = useAuth()
  const { token } = theme.useToken()
  const selected = '/' + (loc.pathname.split('/')[1] || '')
  return (
    <AntLayout style={{ minHeight: '100vh' }}>
      <Sider breakpoint="lg" collapsedWidth={0} theme="dark">
        <div style={{ color: 'white', padding: 16, fontSize: 16, fontWeight: 600 }}>
          proxy_VPN 用户中心
        </div>
        <Menu theme="dark" selectedKeys={[selected]} mode="inline" items={items} />
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
