import { Form, Input, Button, Card, message } from 'antd'
import { useNavigate } from 'react-router-dom'
import { http } from '../api/http'
import { useAuth } from '../store/auth'

function roleFromJWT(token: string): string {
  try {
    const payload = token.split('.')[1]
    const json = JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')))
    return json.role || 'user'
  } catch {
    return 'user'
  }
}

export function Login() {
  const nav = useNavigate()
  const setAuth = useAuth((s) => s.setAuth)
  const onFinish = async (v: { email: string; password: string }) => {
    try {
      const r = await http.post<any>('/auth/login', v)
      const data: any = r.data
      const token: string = data.access_token
      const role = roleFromJWT(token)
      if (!['admin', 'ops', 'finance'].includes(role)) {
        message.error('该账号无管理员权限')
        return
      }
      setAuth(token, v.email, role)
      nav('/')
    } catch (e: any) {
      message.error(e.message || '登录失败')
    }
  }
  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f0f2f5' }}>
      <Card title="proxy_VPN 管理后台" style={{ width: 400 }}>
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
        </Form>
      </Card>
    </div>
  )
}
