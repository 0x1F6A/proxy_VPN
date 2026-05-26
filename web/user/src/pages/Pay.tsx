import { Card, Form, Radio, Button, Spin, Result, message, Typography, Tag, Descriptions } from 'antd'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { useState, useEffect } from 'react'
import { QRCodeSVG } from 'qrcode.react'
import { http } from '../api/http'

interface PayResp {
  id: number
  order_no: string
  channel: string
  qr_or_url: string
  address: string
  amount_cny: string
  amount_token: string
  expired_at: string
  status: string
}
interface Order {
  OrderNo: string
  AmountCNY: string
  PaidCNY: string
  PayMethod: string
  Status: string
  Type: string
}

export function Pay() {
  const { no } = useParams<{ no: string }>()
  const nav = useNavigate()
  const [pay, setPay] = useState<PayResp | null>(null)

  const order = useQuery({
    queryKey: ['order', no],
    queryFn: async () => (await http.get<Order>(`/orders/${no}`)).data,
    refetchInterval: pay ? 3000 : false,
  })

  useEffect(() => {
    if (order.data?.Status === 'paid') {
      message.success('支付成功')
      setTimeout(() => nav('/orders'), 800)
    }
  }, [order.data?.Status, nav])

  const createPay = useMutation({
    mutationFn: async (channel: string) =>
      (await http.post<PayResp>(`/orders/${no}/pay`, { channel })).data,
    onSuccess: (data) => setPay(data),
    onError: (e: any) => message.error(e.message || '支付创建失败'),
  })

  const mockPay = useMutation({
    mutationFn: async () => http.post(`/orders/${no}/mock-pay`),
    onSuccess: () => {
      message.success('已模拟支付成功')
      order.refetch()
    },
  })

  if (order.isLoading) return <Spin />
  const o = order.data

  if (o?.Status === 'paid') {
    return (
      <Result
        status="success"
        title="支付成功"
        subTitle={`订单 ${no} 已完成`}
        extra={[
          <Button type="primary" key="orders">
            <Link to="/orders">我的订单</Link>
          </Button>,
          <Button key="dashboard">
            <Link to="/">回到仪表盘</Link>
          </Button>,
        ]}
      />
    )
  }

  if (o?.Status === 'cancelled' || o?.Status === 'expired') {
    return (
      <Result status="warning" title={`订单已${o.Status === 'cancelled' ? '取消' : '过期'}`}
        extra={<Button type="primary"><Link to="/plans">重新下单</Link></Button>} />
    )
  }

  return (
    <Card title={`支付订单 ${no}`}>
      <Descriptions column={2} size="small" style={{ marginBottom: 16 }}>
        <Descriptions.Item label="订单号">{o?.OrderNo}</Descriptions.Item>
        <Descriptions.Item label="类型">{o?.Type}</Descriptions.Item>
        <Descriptions.Item label="原价">¥{o?.AmountCNY}</Descriptions.Item>
        <Descriptions.Item label="实付"><b>¥{o?.PaidCNY}</b></Descriptions.Item>
        <Descriptions.Item label="状态"><Tag color="orange">{o?.Status}</Tag></Descriptions.Item>
      </Descriptions>

      {!pay && (
        <Form
          layout="vertical"
          initialValues={{ channel: o?.PayMethod === 'mock' ? 'mock' : 'alipay' }}
          onFinish={(v) => createPay.mutate(v.channel)}
        >
          <Form.Item name="channel" label="选择支付渠道">
            <Radio.Group>
              <Radio.Button value="alipay">支付宝</Radio.Button>
              <Radio.Button value="wechat">微信</Radio.Button>
              <Radio.Button value="usdt_trc20">USDT (TRC20)</Radio.Button>
              <Radio.Button value="mock">模拟（开发）</Radio.Button>
            </Radio.Group>
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={createPay.isPending}>
            生成支付
          </Button>
        </Form>
      )}

      {pay && (
        <div style={{ textAlign: 'center', padding: 24 }}>
          {pay.channel === 'usdt_trc20' ? (
            <>
              <Typography.Title level={4}>USDT (TRC20) 收款</Typography.Title>
              <div style={{ margin: '16px 0' }}>
                <QRCodeSVG value={pay.address} size={220} />
              </div>
              <Typography.Paragraph copyable>{pay.address}</Typography.Paragraph>
              <Typography.Text>
                请精确转账 <b>{pay.amount_token} USDT</b>（金额必须完全一致，否则系统无法识别）
              </Typography.Text>
            </>
          ) : (
            <>
              <Typography.Title level={4}>扫码支付 ¥{pay.amount_cny}</Typography.Title>
              <div style={{ margin: '16px 0' }}>
                <QRCodeSVG value={pay.qr_or_url} size={220} />
              </div>
              <Typography.Paragraph copyable>{pay.qr_or_url}</Typography.Paragraph>
            </>
          )}
          <div style={{ marginTop: 16, color: '#888' }}>
            页面将在支付成功后自动跳转，无需手动刷新
          </div>
          {pay.channel === 'mock' && (
            <Button style={{ marginTop: 16 }} onClick={() => mockPay.mutate()} loading={mockPay.isPending}>
              触发模拟支付完成
            </Button>
          )}
        </div>
      )}
    </Card>
  )
}
