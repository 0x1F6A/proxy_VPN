import { Table, Button, Space, Tag, message, Input } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { http } from '../api/http'

interface User {
  id: number
  email: string
  role: string
  status: number
  plan_id: number | null
  traffic_total: number
  traffic_used: number
  created_at: string
}

export function Users() {
  const qc = useQueryClient()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const q = useQuery({
    queryKey: ['admin', 'users', page, pageSize, keyword],
    queryFn: async () =>
      (
        await http.get<any>('/admin/users', {
          params: { page, page_size: pageSize, keyword },
        })
      ).data,
  })
  const ban = useMutation({
    mutationFn: async (id: number) => http.post(`/admin/users/${id}/ban`, { reason: 'admin-ui' }),
    onSuccess: () => {
      message.success('已封禁')
      qc.invalidateQueries({ queryKey: ['admin', 'users'] })
    },
    onError: (e: Error) => message.error(e.message),
  })
  const data: any = q.data || {}
  const items: User[] = data.items || data.list || []
  return (
    <>
      <Space style={{ marginBottom: 16 }}>
        <Input.Search
          placeholder="邮箱搜索"
          allowClear
          onSearch={(v) => {
            setKeyword(v)
            setPage(1)
          }}
          style={{ width: 280 }}
        />
      </Space>
      <Table<User>
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
          { title: '邮箱', dataIndex: 'email' },
          {
            title: '角色',
            dataIndex: 'role',
            width: 120,
            render: (r) => <Tag color={r === 'admin' ? 'red' : r === 'user' ? 'blue' : 'gold'}>{r}</Tag>,
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 100,
            render: (s) =>
              s === 1 ? <Tag color="green">正常</Tag> : s === 0 ? <Tag>已禁用</Tag> : <Tag color="orange">待激活</Tag>,
          },
          { title: '套餐', dataIndex: 'plan_id', width: 100, render: (p) => p ?? '-' },
          {
            title: '流量',
            width: 180,
            render: (_, r) =>
              `${(Number(r.traffic_used) / 1e9).toFixed(2)} / ${(Number(r.traffic_total) / 1e9).toFixed(2)} GB`,
          },
          { title: '注册时间', dataIndex: 'created_at', width: 180 },
          {
            title: '操作',
            width: 120,
            render: (_, r) => (
              <Button danger size="small" onClick={() => ban.mutate(r.id)} disabled={r.status === 0}>
                封禁
              </Button>
            ),
          },
        ]}
      />
    </>
  )
}
