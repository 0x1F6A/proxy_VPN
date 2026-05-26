import { Card, Form, Input, Button, message, Descriptions, Modal, Image, Typography, Space, Tag } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { http } from '../api/http'

interface Me {
  email: string
  uuid: string
  totp_enabled: boolean
  created_at: string
  last_login_at: string | null
}

export function Profile() {
  const qc = useQueryClient()
  const { data } = useQuery({
    queryKey: ['me'],
    queryFn: async () => (await http.get<Me>('/user/me')).data,
  })

  const changePwd = useMutation({
    mutationFn: async (v: any) => http.post('/user/password', v),
    onSuccess: () => message.success('密码已修改'),
    onError: (e: any) => message.error(e.message || '修改失败'),
  })

  const [totpOpen, setTotpOpen] = useState(false)
  const [enrollment, setEnrollment] = useState<{ secret: string; otpauth: string; qr_png_b64: string } | null>(null)

  const enroll = useMutation({
    mutationFn: async () => (await http.post<any>('/user/2fa/enroll', {})).data,
    onSuccess: (d) => {
      setEnrollment(d)
      setTotpOpen(true)
    },
  })
  const verify = useMutation({
    mutationFn: async (code: string) => http.post('/user/2fa/verify', { code }),
    onSuccess: () => {
      message.success('2FA 已启用')
      setTotpOpen(false)
      qc.invalidateQueries({ queryKey: ['me'] })
    },
    onError: (e: any) => message.error(e.message || '验证失败'),
  })
  const disable = useMutation({
    mutationFn: async (code: string) => http.post('/user/2fa/disable', { code }),
    onSuccess: () => {
      message.success('2FA 已关闭')
      qc.invalidateQueries({ queryKey: ['me'] })
    },
    onError: (e: any) => message.error(e.message || '关闭失败'),
  })

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      <Card title="账户信息">
        <Descriptions column={1} size="small">
          <Descriptions.Item label="邮箱">{data?.email}</Descriptions.Item>
          <Descriptions.Item label="代理 UUID"><code>{data?.uuid}</code></Descriptions.Item>
          <Descriptions.Item label="2FA">
            {data?.totp_enabled ? <Tag color="green">已启用</Tag> : <Tag>未启用</Tag>}
          </Descriptions.Item>
          <Descriptions.Item label="注册时间">{data?.created_at && new Date(data.created_at).toLocaleString()}</Descriptions.Item>
          <Descriptions.Item label="最后登录">{data?.last_login_at ? new Date(data.last_login_at).toLocaleString() : '—'}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title="修改密码">
        <Form layout="vertical" onFinish={(v) => changePwd.mutate(v)} style={{ maxWidth: 400 }}>
          <Form.Item name="old_password" label="当前密码" rules={[{ required: true }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="new_password" label="新密码 (≥8 位)" rules={[{ required: true, min: 8 }]}>
            <Input.Password />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={changePwd.isPending}>
            提交
          </Button>
        </Form>
      </Card>

      <Card title="两步验证 (TOTP)">
        {data?.totp_enabled ? (
          <Form layout="inline" onFinish={(v) => disable.mutate(v.code)}>
            <Form.Item name="code" rules={[{ required: true, len: 6 }]}>
              <Input placeholder="6 位 TOTP 码" />
            </Form.Item>
            <Button danger htmlType="submit" loading={disable.isPending}>
              关闭 2FA
            </Button>
          </Form>
        ) : (
          <Button type="primary" onClick={() => enroll.mutate()} loading={enroll.isPending}>
            启用 2FA
          </Button>
        )}
      </Card>

      <Modal title="扫描以下二维码绑定" open={totpOpen} onCancel={() => setTotpOpen(false)} footer={null}>
        {enrollment && (
          <Space direction="vertical" style={{ width: '100%' }}>
            <div style={{ textAlign: 'center' }}>
              <Image src={`data:image/png;base64,${enrollment.qr_png_b64}`} width={200} preview={false} />
            </div>
            <Typography.Paragraph copyable code style={{ wordBreak: 'break-all' }}>
              {enrollment.otpauth}
            </Typography.Paragraph>
            <Typography.Text>密钥: <code>{enrollment.secret}</code></Typography.Text>
            <Form layout="inline" onFinish={(v) => verify.mutate(v.code)}>
              <Form.Item name="code" rules={[{ required: true, len: 6 }]}>
                <Input placeholder="输入 App 上的 6 位码" autoFocus />
              </Form.Item>
              <Button type="primary" htmlType="submit" loading={verify.isPending}>
                验证并启用
              </Button>
            </Form>
          </Space>
        )}
      </Modal>
    </Space>
  )
}
