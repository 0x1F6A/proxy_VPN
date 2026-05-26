import { Table } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { http } from '../api/http'

interface Order {
  order_no: string
  user_id: number
  type: string
  status: string
  amount_cny: string
  pay_method: string
  created_at: string
}

export function Orders() {
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const q = useQuery({
    queryKey: ['admin', 'orders', page, pageSize],
    queryFn: async () =>
      (await http.get<any>('/admin/orders', { params: { page, page_size: pageSize } })).data,
  })
  const data: any = q.data || {}
  const items: Order[] = data.items || data.list || []
  return (
    <Table<Order>
      rowKey="order_no"
      loading={q.isLoading}
      dataSource={items}
      pagination={{
        current: page,
        pageSize,
        total: data.total ?? 0,
        showSizeChanger: true,
        onChange: (p, ps) => {
          setPage(p)
          setPageSize(ps)
        },
      }}
      columns={[
        { title: '订单号', dataIndex: 'order_no', width: 220 },
        { title: '用户', dataIndex: 'user_id', width: 100 },
        { title: '类型', dataIndex: 'type', width: 100 },
        { title: '状态', dataIndex: 'status', width: 100 },
        { title: '金额(CNY)', dataIndex: 'amount_cny', width: 120 },
        { title: '支付方式', dataIndex: 'pay_method', width: 120 },
        { title: '创建时间', dataIndex: 'created_at', width: 180 },
      ]}
    />
  )
}
