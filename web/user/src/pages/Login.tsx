import { Form, Input, Button, Card, message, Typography } from 'antd'
import { Link, useNavigate } from 'react-router-dom'
import { http } from '../api/http'
import { useAuth } from '../store/auth'

export function Login() {
  const nav = useNavigate()
  const setAuth = useAuth((s) => s.setAuth)
  const onFinish = async (v: { email: string; password: string }) => {
    try {
      const r = await http.post<any>('/auth/login', v)
      setAuth(r.data.access_token, v.email)
      nav('/')
    } catch (e: any) {
      message.error(e.message || '登录失败')
    }
  }
  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f0f2f5' }}>
      <Card title="proxy_VPN 用户登录" style={{ width: 400 }}>
        <Form layout="vertical" onFinish={onFinish}>
          <Form.Item name="email" label="邮箱" rules={[{ required: true, type: 'email' }]}>
            <Input autoFocus />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true }]}>
            <Input.Password />
          </Form.Item>
          <Button type="primary" htmlType="submit" block>
            登录
          </Button>
          <Typography.Paragraph style={{ marginTop: 16, textAlign: 'center', marginBottom: 0 }}>
            还没有账号？<Link to="/register">立即注册</Link>
          </Typography.Paragraph>
        </Form>
      </Card>
    </div>
  )
}
