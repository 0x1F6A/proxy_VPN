import { Table, Button, Drawer, Form, Input, InputNumber, message, Space, Popconfirm } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { http } from '../api/http'

interface Coupon {
  id: number
  code: string
  type: string
  value: string
  max_uses: number
  used: number
  status: number
  expires_at: string | null
}

export function Coupons() {
  const qc = useQueryClient()
  const [editing, setEditing] = useState<Coupon | null>(null)
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)

  const q = useQuery({
    queryKey: ['admin', 'coupons', page, pageSize],
    queryFn: async () =>
      (await http.get<any>('/admin/coupons', { params: { page, page_size: pageSize } })).data,
  })

  const save = useMutation({
    mutationFn: async (v: any) => {
      if (editing) return http.put(`/admin/coupons/${editing.id}`, v)
      return http.post('/admin/coupons', v)
    },
    onSuccess: () => {
      message.success('已保存')
      setOpen(false)
      qc.invalidateQueries({ queryKey: ['admin', 'coupons'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const del = useMutation({
    mutationFn: async (id: number) => http.delete(`/admin/coupons/${id}`),
    onSuccess: () => {
      message.success('已删除')
      qc.invalidateQueries({ queryKey: ['admin', 'coupons'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const data: any = q.data || {}
  const items: Coupon[] = data.items || data.list || []
  return (
    <>
      <Space style={{ marginBottom: 16 }}>
        <Button
          type="primary"
          onClick={() => {
            setEditing(null)
            form.resetFields()
            setOpen(true)
          }}
        >
          新增优惠券
        </Button>
      </Space>
      <Table<Coupon>
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
          { title: '券码', dataIndex: 'code' },
          { title: '类型', dataIndex: 'type', width: 100 },
          { title: '值', dataIndex: 'value', width: 100 },
          { title: '可用次数', dataIndex: 'max_uses', width: 100 },
          { title: '已用', dataIndex: 'used', width: 80 },
          { title: '状态', dataIndex: 'status', width: 80 },
          { title: '过期', dataIndex: 'expires_at', width: 180 },
          {
            title: '操作',
            width: 180,
            render: (_, r) => (
              <Space>
                <Button
                  size="small"
                  onClick={() => {
                    setEditing(r)
                    form.setFieldsValue(r)
                    setOpen(true)
                  }}
                >
                  编辑
                </Button>
                <Popconfirm title="确认删除？" onConfirm={() => del.mutate(r.id)}>
                  <Button danger size="small">
                    删除
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
      <Drawer title={editing ? '编辑优惠券' : '新增优惠券'} open={open} onClose={() => setOpen(false)} width={420}>
        <Form form={form} layout="vertical" onFinish={(v) => save.mutate(v)}>
          <Form.Item name="code" label="券码" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]} initialValue="amount">
            <Input placeholder="amount / percent" />
          </Form.Item>
          <Form.Item name="value" label="值" rules={[{ required: true }]}>
            <Input placeholder="20 表示折 20 元 或 20%" />
          </Form.Item>
          <Form.Item name="max_uses" label="总次数" rules={[{ required: true }]} initialValue={100}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="status" label="状态" initialValue={1}>
            <InputNumber min={0} max={1} style={{ width: '100%' }} />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={save.isPending}>
            保存
          </Button>
        </Form>
      </Drawer>
    </>
  )
}
