import { Table, Tag, Button, Space, message, Popconfirm } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { http } from '../api/http'

interface Order {
  ID: number
  OrderNo: string
  Type: string
  AmountCNY: string
  PaidCNY: string
  PayMethod: string
  Status: string
  CreatedAt: string
  ExpireAt: string
}

const statusColor: Record<string, string> = {
  pending: 'orange',
  paid: 'green',
  cancelled: 'default',
  expired: 'red',
  refunded: 'purple',
}
const statusLabel: Record<string, string> = {
  pending: '待支付',
  paid: '已支付',
  cancelled: '已取消',
  expired: '已过期',
  refunded: '已退款',
}

export function Orders() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['my-orders'],
    queryFn: async () => (await http.get<Order[]>('/orders?limit=50')).data,
  })

  const cancel = useMutation({
    mutationFn: async (no: string) => http.post(`/orders/${no}/cancel`),
    onSuccess: () => {
      message.success('已取消')
      qc.invalidateQueries({ queryKey: ['my-orders'] })
    },
    onError: (e: any) => message.error(e.message || '取消失败'),
  })

  return (
    <Table
      rowKey="ID"
      loading={isLoading}
      dataSource={data || []}
      pagination={{ pageSize: 20 }}
      columns={[
        { title: '订单号', dataIndex: 'OrderNo', width: 220 },
        { title: '类型', dataIndex: 'Type', width: 90, render: (t) => ({ plan: '套餐', pack: '流量包', topup: '充值' }[t as string] || t) },
        { title: '金额', dataIndex: 'AmountCNY', width: 100, render: (v) => `¥${v}` },
        { title: '实付', dataIndex: 'PaidCNY', width: 100, render: (v) => `¥${v}` },
        { title: '渠道', dataIndex: 'PayMethod', width: 110 },
        {
          title: '状态',
          dataIndex: 'Status',
          width: 100,
          render: (s) => <Tag color={statusColor[s] || 'default'}>{statusLabel[s] || s}</Tag>,
        },
        { title: '创建时间', dataIndex: 'CreatedAt', width: 180, render: (v) => new Date(v).toLocaleString() },
        {
          title: '操作',
          fixed: 'right',
          width: 180,
          render: (_, r) => (
            <Space>
              {r.Status === 'pending' && (
                <>
                  <Button size="small" type="primary">
                    <Link to={`/pay/${r.OrderNo}`}>去支付</Link>
                  </Button>
                  <Popconfirm title="取消订单？" onConfirm={() => cancel.mutate(r.OrderNo)}>
                    <Button size="small" danger>
                      取消
                    </Button>
                  </Popconfirm>
                </>
              )}
            </Space>
          ),
        },
      ]}
      scroll={{ x: 1100 }}
    />
  )
}
