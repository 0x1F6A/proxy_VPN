import { Card, Row, Col, Progress, Statistic, Tag, Skeleton, Button } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { http } from '../api/http'

interface Me {
  email: string
  balance_cny: number
  plan_id: number | null
  plan_expire_at: string | null
  traffic_total: number
  traffic_used: number
  traffic_reset_at: string | null
  device_limit: number
  invite_code: string
}

function fmtBytes(n: number): string {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(2)} ${u[i]}`
}

export function Dashboard() {
  const { data, isLoading } = useQuery({
    queryKey: ['me'],
    queryFn: async () => (await http.get<Me>('/user/me')).data,
  })

  if (isLoading || !data) return <Skeleton active />

  const used = Number(data.traffic_used || 0)
  const total = Number(data.traffic_total || 0)
  const pct = total ? Math.min(100, Math.round((used / total) * 100)) : 0
  const expire = data.plan_expire_at ? new Date(data.plan_expire_at) : null
  const daysLeft = expire ? Math.max(0, Math.ceil((expire.getTime() - Date.now()) / 86400000)) : 0

  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} md={8}>
        <Card>
          <Statistic title="账户余额 (CNY)" value={Number(data.balance_cny || 0)} precision={2} prefix="¥" />
        </Card>
      </Col>
      <Col xs={24} md={8}>
        <Card>
          <Statistic
            title="套餐状态"
            value={data.plan_id ? `剩余 ${daysLeft} 天` : '未订阅'}
            valueStyle={{ color: data.plan_id ? (daysLeft <= 7 ? '#cf1322' : '#3f8600') : '#999' }}
          />
          {expire && (
            <div style={{ marginTop: 8, color: '#888', fontSize: 12 }}>
              到期: {expire.toLocaleString()}
            </div>
          )}
          <Button type="primary" size="small" style={{ marginTop: 12 }}>
            <Link to="/plans">购买 / 续费</Link>
          </Button>
        </Card>
      </Col>
      <Col xs={24} md={8}>
        <Card>
          <Statistic title="设备数上限" value={data.device_limit} suffix="台" />
          <div style={{ marginTop: 8, color: '#888', fontSize: 12 }}>
            邀请码 <Tag color="blue">{data.invite_code}</Tag>
          </div>
        </Card>
      </Col>

      <Col xs={24}>
        <Card title="本期流量">
          <Progress
            percent={pct}
            status={pct >= 95 ? 'exception' : pct >= 80 ? 'active' : 'normal'}
            format={() => `${fmtBytes(used)} / ${fmtBytes(total) || '∞'}`}
          />
          {data.traffic_reset_at && (
            <div style={{ marginTop: 8, color: '#888', fontSize: 12 }}>
              下次重置: {new Date(data.traffic_reset_at).toLocaleString()}
            </div>
          )}
        </Card>
      </Col>

      <Col xs={24}>
        <Card title="快速入口">
          <Button type="primary" style={{ marginRight: 8 }}>
            <Link to="/subscription">复制订阅链接</Link>
          </Button>
          <Button style={{ marginRight: 8 }}>
            <Link to="/nodes">查看节点</Link>
          </Button>
          <Button style={{ marginRight: 8 }}>
            <Link to="/orders">我的订单</Link>
          </Button>
          <Button>
            <Link to="/help">客户端下载</Link>
          </Button>
        </Card>
      </Col>
    </Row>
  )
}
