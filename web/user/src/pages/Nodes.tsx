import { Table, Tag, Badge, Alert } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { http } from '../api/http'

interface Node {
  id: number
  name: string
  region: string
  tags: string
  protocol: string
  online: boolean
  rate_multiplier: string
  sort: number
}

export function Nodes() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['my-nodes'],
    queryFn: async () => (await http.get<Node[]>('/nodes')).data,
    refetchInterval: 30000,
  })

  if (error) {
    return <Alert type="info" message="尚未开通套餐时无法查看节点。请先到「套餐」页购买。" />
  }

  return (
    <Table
      rowKey="id"
      loading={isLoading}
      dataSource={data || []}
      pagination={{ pageSize: 20 }}
      columns={[
        { title: '名称', dataIndex: 'name' },
        { title: '区域', dataIndex: 'region', width: 110, render: (v) => <Tag color="geekblue">{v}</Tag> },
        { title: '协议', dataIndex: 'protocol', width: 110 },
        { title: '倍率', dataIndex: 'rate_multiplier', width: 80, render: (v) => `${v}x` },
        { title: '标签', dataIndex: 'tags', width: 200 },
        {
          title: '在线',
          dataIndex: 'online',
          width: 90,
          render: (v) => <Badge status={v ? 'success' : 'error'} text={v ? '在线' : '离线'} />,
        },
      ]}
    />
  )
}
