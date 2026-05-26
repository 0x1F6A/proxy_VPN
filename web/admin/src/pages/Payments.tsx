import { Table } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { http } from '../api/http'

interface Payment {
  id: number
  order_no: string
  user_id: number
  channel: string
  status: string
  amount_cny: string
  amount_token: string
  tx_hash: string | null
  created_at: string
}

export function Payments() {
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const q = useQuery({
    queryKey: ['admin', 'payments', page, pageSize],
    queryFn: async () =>
      (await http.get<any>('/admin/payments', { params: { page, page_size: pageSize } })).data,
  })
  const data: any = q.data || {}
  const items: Payment[] = data.items || data.list || []
  return (
    <Table<Payment>
      rowKey="id"
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
        { title: 'ID', dataIndex: 'id', width: 80 },
        { title: '订单号', dataIndex: 'order_no', width: 220 },
        { title: '用户', dataIndex: 'user_id', width: 100 },
        { title: '渠道', dataIndex: 'channel', width: 100 },
        { title: '状态', dataIndex: 'status', width: 100 },
        { title: 'CNY', dataIndex: 'amount_cny', width: 120 },
        { title: 'Token', dataIndex: 'amount_token', width: 120 },
        { title: 'TxHash', dataIndex: 'tx_hash', ellipsis: true },
        { title: '创建时间', dataIndex: 'created_at', width: 180 },
      ]}
    />
  )
}
