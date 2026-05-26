import { Card, Row, Col, Button, Tag, Modal, Form, Radio, Select, Input, message, Tabs, Skeleton } from 'antd'
import { useQuery, useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { http } from '../api/http'

interface Plan {
  ID: number
  Name: string
  Description: string
  PriceCNY: string
  DurationDays: number
  TrafficGB: number
  DeviceLimit: number
  SpeedLimitMbps: number
  Status: number
}
interface DataPack {
  ID: number
  Name: string
  PriceCNY: string
  TrafficGB: number
  ValidDays: number
  Status: number
}

function PlanList({ onBuy }: { onBuy: (id: number, type: 'plan' | 'pack') => void }) {
  const { data, isLoading } = useQuery({
    queryKey: ['plans'],
    queryFn: async () => (await http.get<Plan[]>('/plans')).data,
  })
  if (isLoading) return <Skeleton active />
  const list = (data || []).filter((p) => p.Status === 1)
  if (!list.length) return <div style={{ color: '#888' }}>暂无可用套餐</div>
  return (
    <Row gutter={[16, 16]}>
      {list.map((p) => (
        <Col xs={24} sm={12} md={8} key={p.ID}>
          <Card title={p.Name} extra={<Tag color="blue">月付</Tag>} hoverable>
            <div style={{ fontSize: 32, fontWeight: 700, color: '#1677ff' }}>¥{p.PriceCNY}</div>
            <div style={{ color: '#888', marginBottom: 12 }}>{p.Description || '—'}</div>
            <div style={{ fontSize: 13, lineHeight: 1.9 }}>
              <div>📅 时长 <b>{p.DurationDays}</b> 天</div>
              <div>📊 流量 <b>{p.TrafficGB}</b> GB</div>
              <div>📱 设备数 <b>{p.DeviceLimit}</b> 台</div>
              {p.SpeedLimitMbps > 0 && <div>⚡ 限速 {p.SpeedLimitMbps} Mbps</div>}
            </div>
            <Button type="primary" block style={{ marginTop: 12 }} onClick={() => onBuy(p.ID, 'plan')}>
              立即购买
            </Button>
          </Card>
        </Col>
      ))}
    </Row>
  )
}

function PackList({ onBuy }: { onBuy: (id: number, type: 'plan' | 'pack') => void }) {
  const { data, isLoading } = useQuery({
    queryKey: ['packs'],
    queryFn: async () => (await http.get<DataPack[]>('/data-packs')).data,
  })
  if (isLoading) return <Skeleton active />
  const list = (data || []).filter((p) => p.Status === 1)
  if (!list.length) return <div style={{ color: '#888' }}>暂无可用流量包</div>
  return (
    <Row gutter={[16, 16]}>
      {list.map((p) => (
        <Col xs={24} sm={12} md={8} key={p.ID}>
          <Card title={p.Name} extra={<Tag color="purple">流量包</Tag>} hoverable>
            <div style={{ fontSize: 32, fontWeight: 700, color: '#722ed1' }}>¥{p.PriceCNY}</div>
            <div style={{ fontSize: 13, lineHeight: 1.9 }}>
              <div>📊 流量 <b>{p.TrafficGB}</b> GB</div>
              <div>📅 有效期 <b>{p.ValidDays}</b> 天</div>
            </div>
            <Button type="primary" block style={{ marginTop: 12 }} onClick={() => onBuy(p.ID, 'pack')}>
              立即购买
            </Button>
          </Card>
        </Col>
      ))}
    </Row>
  )
}

export function Plans() {
  const [open, setOpen] = useState(false)
  const [target, setTarget] = useState<{ id: number; type: 'plan' | 'pack' } | null>(null)
  const nav = useNavigate()
  const [form] = Form.useForm()

  const createOrder = useMutation({
    mutationFn: async (v: any) =>
      (await http.post<any>('/orders', { ...v, type: target?.type, target_id: target?.id })).data,
    onSuccess: (o) => {
      setOpen(false)
      message.success('订单已创建')
      nav(`/pay/${o.OrderNo}`)
    },
    onError: (e: any) => message.error(e.message || '下单失败'),
  })

  const onBuy = (id: number, type: 'plan' | 'pack') => {
    setTarget({ id, type })
    setOpen(true)
    form.resetFields()
  }

  return (
    <>
      <Tabs
        items={[
          { key: 'plan', label: '订阅套餐', children: <PlanList onBuy={onBuy} /> },
          { key: 'pack', label: '流量包', children: <PackList onBuy={onBuy} /> },
        ]}
      />
      <Modal
        title="确认下单"
        open={open}
        onCancel={() => setOpen(false)}
        onOk={() => form.submit()}
        okText="创建订单"
        confirmLoading={createOrder.isPending}
      >
        <Form form={form} layout="vertical" onFinish={(v) => createOrder.mutate(v)} initialValues={{ pay_method: 'alipay' }}>
          <Form.Item name="pay_method" label="支付方式" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'alipay', label: '支付宝' },
                { value: 'wechat', label: '微信' },
                { value: 'usdt_trc20', label: 'USDT (TRC20)' },
                { value: 'balance', label: '余额抵扣' },
                { value: 'mock', label: '模拟支付（仅开发）' },
              ]}
            />
          </Form.Item>
          <Form.Item name="coupon_code" label="优惠券（选填）">
            <Input placeholder="输入优惠码" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}
