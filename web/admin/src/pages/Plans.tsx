import { Table, Button, Drawer, Form, Input, InputNumber, message, Space, Popconfirm } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { http } from '../api/http'

interface Plan {
  id: number
  name: string
  price_cny: string
  duration_days: number
  traffic_gb: number
  device_limit: number
  status: number
}

export function Plans() {
  const qc = useQueryClient()
  const [editing, setEditing] = useState<Plan | null>(null)
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()

  const q = useQuery({
    queryKey: ['admin', 'plans'],
    queryFn: async () => (await http.get<any>('/admin/plans')).data,
  })

  const save = useMutation({
    mutationFn: async (v: any) => {
      if (editing) return http.put(`/admin/plans/${editing.id}`, v)
      return http.post('/admin/plans', v)
    },
    onSuccess: () => {
      message.success('已保存')
      setOpen(false)
      qc.invalidateQueries({ queryKey: ['admin', 'plans'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const del = useMutation({
    mutationFn: async (id: number) => http.delete(`/admin/plans/${id}`),
    onSuccess: () => {
      message.success('已删除')
      qc.invalidateQueries({ queryKey: ['admin', 'plans'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const items: Plan[] = (q.data as any)?.items || (q.data as any) || []

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
          新增套餐
        </Button>
      </Space>
      <Table<Plan>
        rowKey="id"
        loading={q.isLoading}
        dataSource={items}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 80 },
          { title: '名称', dataIndex: 'name' },
          { title: '价格(CNY)', dataIndex: 'price_cny', width: 120 },
          { title: '天数', dataIndex: 'duration_days', width: 100 },
          { title: '流量(GB)', dataIndex: 'traffic_gb', width: 120 },
          { title: '设备', dataIndex: 'device_limit', width: 100 },
          { title: '状态', dataIndex: 'status', width: 100 },
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
      <Drawer
        title={editing ? '编辑套餐' : '新增套餐'}
        open={open}
        onClose={() => setOpen(false)}
        width={420}
      >
        <Form form={form} layout="vertical" onFinish={(v) => save.mutate(v)}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="price_cny" label="价格(CNY)" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="duration_days" label="天数" rules={[{ required: true }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="traffic_gb" label="流量(GB)" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="device_limit" label="设备数" rules={[{ required: true }]}>
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
