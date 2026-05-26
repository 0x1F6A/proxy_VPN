import { Card, Col, DatePicker, Row, Table, Spin } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import dayjs, { Dayjs } from 'dayjs'
import { http } from '../api/http'

const { RangePicker } = DatePicker

export function Reports() {
  const [range, setRange] = useState<[Dayjs, Dayjs]>([dayjs().subtract(30, 'day'), dayjs()])

  const params = {
    from: range[0].format('YYYY-MM-DD'),
    to: range[1].format('YYYY-MM-DD'),
  }

  const revenue = useQuery({
    queryKey: ['admin', 'reports', 'revenue', params],
    queryFn: async () => (await http.get<any>('/admin/reports/revenue', { params })).data,
  })
  const traffic = useQuery({
    queryKey: ['admin', 'reports', 'traffic', params],
    queryFn: async () => (await http.get<any>('/admin/reports/traffic', { params })).data,
  })
  const orders = useQuery({
    queryKey: ['admin', 'reports', 'orders', params],
    queryFn: async () => (await http.get<any>('/admin/reports/orders', { params })).data,
  })

  const revenueRows = ((revenue.data as any)?.items || (revenue.data as any) || []) as any[]
  const trafficRows = ((traffic.data as any)?.items || (traffic.data as any) || []) as any[]
  const orderRows = ((orders.data as any)?.items || (orders.data as any) || []) as any[]

  return (
    <>
      <RangePicker
        value={range}
        onChange={(r) => {
          if (r && r[0] && r[1]) setRange([r[0], r[1]])
        }}
        style={{ marginBottom: 16 }}
      />
      <Row gutter={16}>
        <Col span={8}>
          <Card title="收入">
            {revenue.isLoading ? (
              <Spin />
            ) : (
              <Table
                size="small"
                rowKey={(r) => r.date}
                dataSource={revenueRows}
                pagination={false}
                columns={[
                  { title: '日期', dataIndex: 'date' },
                  { title: 'CNY', dataIndex: 'amount_cny' },
                ]}
              />
            )}
          </Card>
        </Col>
        <Col span={8}>
          <Card title="流量">
            {traffic.isLoading ? (
              <Spin />
            ) : (
              <Table
                size="small"
                rowKey={(r) => r.date}
                dataSource={trafficRows}
                pagination={false}
                columns={[
                  { title: '日期', dataIndex: 'date' },
                  { title: 'GB', dataIndex: 'gb' },
                ]}
              />
            )}
          </Card>
        </Col>
        <Col span={8}>
          <Card title="订单">
            {orders.isLoading ? (
              <Spin />
            ) : (
              <Table
                size="small"
                rowKey={(r) => r.date}
                dataSource={orderRows}
                pagination={false}
                columns={[
                  { title: '日期', dataIndex: 'date' },
                  { title: '数量', dataIndex: 'count' },
                ]}
              />
            )}
          </Card>
        </Col>
      </Row>
    </>
  )
}
