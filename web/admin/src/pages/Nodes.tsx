import { Table, Button, Drawer, Form, Input, InputNumber, message, Space, Popconfirm, Tag } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { http } from '../api/http'

interface Node {
  id: number
  name: string
  host: string
  port: number
  protocol: string
  group_id: number | null
  status: number
  last_seen_at: string | null
}

export function Nodes() {
  const qc = useQueryClient()
  const [editing, setEditing] = useState<Node | null>(null)
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()

  const q = useQuery({
    queryKey: ['admin', 'nodes'],
    queryFn: async () => (await http.get<any>('/admin/nodes')).data,
  })

  const save = useMutation({
    mutationFn: async (v: any) => {
      if (editing) return http.put(`/admin/nodes/${editing.id}`, v)
      return http.post('/admin/nodes', v)
    },
    onSuccess: () => {
      message.success('已保存')
      setOpen(false)
      qc.invalidateQueries({ queryKey: ['admin', 'nodes'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const del = useMutation({
    mutationFn: async (id: number) => http.delete(`/admin/nodes/${id}`),
    onSuccess: () => {
      message.success('已删除')
      qc.invalidateQueries({ queryKey: ['admin', 'nodes'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const items: Node[] = (q.data as any)?.items || (q.data as any) || []

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
          新增节点
        </Button>
      </Space>
      <Table<Node>
        rowKey="id"
        loading={q.isLoading}
        dataSource={items}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 80 },
          { title: '名称', dataIndex: 'name' },
          { title: '地址', render: (_, r) => `${r.host}:${r.port}` },
          { title: '协议', dataIndex: 'protocol', width: 100 },
          { title: '分组', dataIndex: 'group_id', width: 100 },
          {
            title: '状态',
            dataIndex: 'status',
            width: 100,
            render: (s) => (s === 1 ? <Tag color="green">在线</Tag> : <Tag>下线</Tag>),
          },
          { title: '心跳', dataIndex: 'last_seen_at', width: 180 },
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
      <Drawer title={editing ? '编辑节点' : '新增节点'} open={open} onClose={() => setOpen(false)} width={420}>
        <Form form={form} layout="vertical" onFinish={(v) => save.mutate(v)}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="host" label="Host" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="port" label="Port" rules={[{ required: true }]}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="protocol" label="协议" rules={[{ required: true }]} initialValue="vless">
            <Input />
          </Form.Item>
          <Form.Item name="group_id" label="分组 ID">
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
