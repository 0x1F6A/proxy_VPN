import { Card, Typography, Tag, Alert, Button, message } from 'antd'
import { CopyOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { http } from '../api/http'

interface Me {
  invite_code: string
  email: string
}

export function Invite() {
  const { data } = useQuery({
    queryKey: ['me'],
    queryFn: async () => (await http.get<Me>('/user/me')).data,
  })

  const link = data ? `${location.origin}/register?invite=${data.invite_code}` : ''
  const copy = (s: string) => {
    navigator.clipboard.writeText(s)
    message.success('已复制')
  }

  return (
    <>
      <Card title="我的邀请码">
        <Typography.Title level={2} copyable={{ tooltips: ['复制', '已复制'] }}>
          <Tag color="gold" style={{ fontSize: 24, padding: '4px 16px' }}>
            {data?.invite_code || '—'}
          </Tag>
        </Typography.Title>
        <Typography.Paragraph>
          推荐链接（用户通过此链接注册，邀请关系自动绑定）：
        </Typography.Paragraph>
        <Typography.Paragraph copyable code style={{ wordBreak: 'break-all' }}>
          {link}
        </Typography.Paragraph>
        <Button type="primary" icon={<CopyOutlined />} onClick={() => copy(link)}>
          复制推荐链接
        </Button>
      </Card>

      <Alert
        style={{ marginTop: 16 }}
        type="info"
        showIcon
        message="返佣体系即将上线"
        description="被邀请人完成首次付款后，将自动按当前返佣比例返到你的账户余额，可直接抵扣下次续费。"
      />
    </>
  )
}
