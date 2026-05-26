import { Table, Button, Drawer, Form, Input, InputNumber, message, Space, Popconfirm } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { http } from '../api/http'

interface Pack {
  id: number
  name: string
  traffic_gb: number
  price_cny: string
  status: number
}

export function DataPacks() {
  const qc = useQueryClient()
  const [editing, setEditing] = useState<Pack | null>(null)
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()

  const q = useQuery({
    queryKey: ['admin', 'data-packs'],
    queryFn: async () => (await http.get<any>('/admin/data-packs')).data,
  })

  const save = useMutation({
    mutationFn: async (v: any) => {
      if (editing) return http.put(`/admin/data-packs/${editing.id}`, v)
      return http.post('/admin/data-packs', v)
    },
    onSuccess: () => {
      message.success('已保存')
      setOpen(false)
      qc.invalidateQueries({ queryKey: ['admin', 'data-packs'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const del = useMutation({
    mutationFn: async (id: number) => http.delete(`/admin/data-packs/${id}`),
    onSuccess: () => {
      message.success('已删除')
      qc.invalidateQueries({ queryKey: ['admin', 'data-packs'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const items: Pack[] = (q.data as any)?.items || (q.data as any) || []

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
          新增流量包
        </Button>
      </Space>
      <Table<Pack>
        rowKey="id"
        loading={q.isLoading}
        dataSource={items}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 80 },
          { title: '名称', dataIndex: 'name' },
          { title: '流量(GB)', dataIndex: 'traffic_gb', width: 120 },
          { title: '价格(CNY)', dataIndex: 'price_cny', width: 120 },
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
      <Drawer title={editing ? '编辑流量包' : '新增流量包'} open={open} onClose={() => setOpen(false)} width={420}>
        <Form form={form} layout="vertical" onFinish={(v) => save.mutate(v)}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="traffic_gb" label="流量(GB)" rules={[{ required: true }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="price_cny" label="价格(CNY)" rules={[{ required: true }]}>
            <Input />
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
