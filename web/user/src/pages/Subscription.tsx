import { Card, Typography, Button, message, Space, Tabs } from 'antd'
import { CopyOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { QRCodeSVG } from 'qrcode.react'
import { http } from '../api/http'

interface Me {
  subscription_token: string
}

export function Subscription() {
  const { data } = useQuery({
    queryKey: ['me'],
    queryFn: async () => (await http.get<Me>('/user/me')).data,
  })

  const tok = data?.subscription_token || ''
  const base = `${location.origin}/sub/${tok}`
  const urls = {
    clash: `${base}?fmt=clash`,
    'sing-box': `${base}?fmt=singbox`,
    'v2ray (base64)': `${base}`,
  }

  const copy = (s: string) => {
    navigator.clipboard.writeText(s)
    message.success('已复制')
  }

  return (
    <Card title="订阅链接">
      <Typography.Paragraph type="secondary">
        把以下链接导入到客户端（Clash / Sing-box / V2RayN / Shadowrocket 等）即可使用。
        请勿外泄链接 —— 等同于账号密码。
      </Typography.Paragraph>

      <Tabs
        items={Object.entries(urls).map(([name, url]) => ({
          key: name,
          label: name,
          children: (
            <Space direction="vertical" style={{ width: '100%' }} size={16}>
              <Typography.Paragraph
                copyable={{ tooltips: ['复制', '已复制'] }}
                code
                style={{ wordBreak: 'break-all', marginBottom: 0 }}
              >
                {url}
              </Typography.Paragraph>
              <div style={{ textAlign: 'center' }}>
                <QRCodeSVG value={url} size={200} />
                <div style={{ marginTop: 8, color: '#888', fontSize: 12 }}>用手机客户端扫码</div>
              </div>
              <Space>
                <Button type="primary" icon={<CopyOutlined />} onClick={() => copy(url)}>
                  复制链接
                </Button>
                <Button
                  onClick={() => {
                    navigator.clipboard.writeText(tok)
                    message.success('Token 已复制')
                  }}
                >
                  仅复制 Token
                </Button>
              </Space>
            </Space>
          ),
        }))}
      />
    </Card>
  )
}
