import { Form, Input, Button, Card, message, Typography, Space } from 'antd'
import { Link, useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { http } from '../api/http'

export function Register() {
  const nav = useNavigate()
  const [sending, setSending] = useState(false)
  const [countdown, setCountdown] = useState(0)
  const [form] = Form.useForm()

  const sendCode = async () => {
    const email = form.getFieldValue('email')
    if (!email) {
      message.warning('请先填邮箱')
      return
    }
    try {
      setSending(true)
      await http.post('/auth/send-code', { email, scene: 'register' })
      message.success('验证码已发送，请查收邮箱')
      setCountdown(60)
      const t = setInterval(() => {
        setCountdown((c) => {
          if (c <= 1) {
            clearInterval(t)
            return 0
          }
          return c - 1
        })
      }, 1000)
    } catch (e: any) {
      message.error(e.message || '发送失败')
    } finally {
      setSending(false)
    }
  }

  const onFinish = async (v: { email: string; password: string; code: string }) => {
    try {
      await http.post('/auth/register', v)
      message.success('注册成功，请登录')
      nav('/login')
    } catch (e: any) {
      message.error(e.message || '注册失败')
    }
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f0f2f5' }}>
      <Card title="注册 proxy_VPN" style={{ width: 420 }}>
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item name="email" label="邮箱" rules={[{ required: true, type: 'email' }]}>
            <Input />
          </Form.Item>
          <Form.Item label="邮箱验证码" required>
            <Space.Compact style={{ width: '100%' }}>
              <Form.Item name="code" noStyle rules={[{ required: true }]}>
                <Input placeholder="6 位验证码" />
              </Form.Item>
              <Button onClick={sendCode} loading={sending} disabled={countdown > 0}>
                {countdown > 0 ? `${countdown}s` : '发送验证码'}
              </Button>
            </Space.Compact>
          </Form.Item>
          <Form.Item name="password" label="密码（≥8 位）" rules={[{ required: true, min: 8 }]}>
            <Input.Password />
          </Form.Item>
          <Button type="primary" htmlType="submit" block>
            注册
          </Button>
          <Typography.Paragraph style={{ marginTop: 16, textAlign: 'center', marginBottom: 0 }}>
            已有账号？<Link to="/login">直接登录</Link>
          </Typography.Paragraph>
        </Form>
      </Card>
    </div>
  )
}
