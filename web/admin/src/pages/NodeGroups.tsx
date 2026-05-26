import { Table, Button, Drawer, Form, Input, InputNumber, message, Space, Popconfirm } from 'antd'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { http } from '../api/http'

interface Group {
  id: number
  name: string
  level: number
  remark: string
}

export function NodeGroups() {
  const qc = useQueryClient()
  const [editing, setEditing] = useState<Group | null>(null)
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()

  const q = useQuery({
    queryKey: ['admin', 'node-groups'],
    queryFn: async () => (await http.get<any>('/admin/node-groups')).data,
  })

  const save = useMutation({
    mutationFn: async (v: any) => {
      if (editing) return http.put(`/admin/node-groups/${editing.id}`, v)
      return http.post('/admin/node-groups', v)
    },
    onSuccess: () => {
      message.success('已保存')
      setOpen(false)
      qc.invalidateQueries({ queryKey: ['admin', 'node-groups'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const del = useMutation({
    mutationFn: async (id: number) => http.delete(`/admin/node-groups/${id}`),
    onSuccess: () => {
      message.success('已删除')
      qc.invalidateQueries({ queryKey: ['admin', 'node-groups'] })
    },
    onError: (e: Error) => message.error(e.message),
  })

  const items: Group[] = (q.data as any)?.items || (q.data as any) || []

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
          新增分组
        </Button>
      </Space>
      <Table<Group>
        rowKey="id"
        loading={q.isLoading}
        dataSource={items}
        columns={[
          { title: 'ID', dataIndex: 'id', width: 80 },
          { title: '名称', dataIndex: 'name' },
          { title: '等级', dataIndex: 'level', width: 100 },
          { title: '备注', dataIndex: 'remark' },
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
      <Drawer title={editing ? '编辑分组' : '新增分组'} open={open} onClose={() => setOpen(false)} width={420}>
        <Form form={form} layout="vertical" onFinish={(v) => save.mutate(v)}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="level" label="等级" initialValue={1}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={save.isPending}>
            保存
          </Button>
        </Form>
      </Drawer>
    </>
  )
}
