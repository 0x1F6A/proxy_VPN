import { Card, Col, Row, Statistic, Spin, Alert } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { http } from '../api/http'

export function Dashboard() {
  const q = useQuery({
    queryKey: ['admin', 'reports', 'dashboard'],
    queryFn: async () => (await http.get<any>('/admin/reports/dashboard')).data,
  })
  if (q.isLoading) return <Spin />
  if (q.isError) return <Alert type="error" message={(q.error as Error).message} />
  const d: any = q.data || {}
  return (
    <Row gutter={16}>
      <Col span={6}>
        <Card>
          <Statistic title="今日新增用户" value={d.users_today ?? 0} />
        </Card>
      </Col>
      <Col span={6}>
        <Card>
          <Statistic title="今日订单" value={d.orders_today ?? 0} />
        </Card>
      </Col>
      <Col span={6}>
        <Card>
          <Statistic title="今日收入 (CNY)" value={d.revenue_today ?? 0} precision={2} />
        </Card>
      </Col>
      <Col span={6}>
        <Card>
          <Statistic title="在线节点" value={d.nodes_online ?? 0} />
        </Card>
      </Col>
    </Row>
  )
}
