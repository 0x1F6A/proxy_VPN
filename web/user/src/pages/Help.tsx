import { Card, Typography, List, Tag, Space, Divider } from 'antd'

const clients = [
  {
    platform: 'Windows',
    items: [
      { name: 'Clash Verge Rev', url: 'https://github.com/clash-verge-rev/clash-verge-rev/releases' },
      { name: 'V2RayN', url: 'https://github.com/2dust/v2rayN/releases' },
      { name: 'NekoBox', url: 'https://github.com/MatsuriDayo/nekoray/releases' },
    ],
  },
  {
    platform: 'macOS',
    items: [
      { name: 'ClashX Meta', url: 'https://github.com/MetaCubeX/ClashX.Meta/releases' },
      { name: 'Surge', url: 'https://nssurge.com/' },
      { name: 'Quantumult X', url: 'https://apps.apple.com/app/quantumult-x/id1443988620' },
    ],
  },
  {
    platform: 'iOS',
    items: [
      { name: 'Shadowrocket', url: 'https://apps.apple.com/app/shadowrocket/id932747118' },
      { name: 'Stash', url: 'https://apps.apple.com/app/stash/id1596063349' },
      { name: 'Surge iOS', url: 'https://apps.apple.com/app/surge-5/id1442620678' },
    ],
  },
  {
    platform: 'Android',
    items: [
      { name: 'Clash for Android (Meta)', url: 'https://github.com/MetaCubeX/ClashMetaForAndroid/releases' },
      { name: 'V2RayNG', url: 'https://github.com/2dust/v2rayNG/releases' },
      { name: 'NekoBox for Android', url: 'https://github.com/MatsuriDayo/NekoBoxForAndroid/releases' },
    ],
  },
  {
    platform: 'Linux / 路由器',
    items: [
      { name: 'Clash Meta (Mihomo)', url: 'https://github.com/MetaCubeX/mihomo/releases' },
      { name: 'sing-box', url: 'https://github.com/SagerNet/sing-box/releases' },
    ],
  },
]

export function Help() {
  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      <Card title="新手三步走">
        <Typography.Paragraph>
          <b>1. 在「订阅」页复制订阅链接</b>（推荐 Clash 格式）<br />
          <b>2. 下载下方对应平台的客户端</b><br />
          <b>3. 在客户端「订阅」/「配置组」里粘贴链接，更新即可</b>
        </Typography.Paragraph>
        <Typography.Paragraph type="secondary">
          首次使用建议选择「自动选择」节点；若某条线路速度慢，可手动切换其它节点。
        </Typography.Paragraph>
      </Card>

      <Card title="客户端下载">
        {clients.map((c) => (
          <div key={c.platform} style={{ marginBottom: 16 }}>
            <Tag color="blue">{c.platform}</Tag>
            <List
              size="small"
              dataSource={c.items}
              renderItem={(it) => (
                <List.Item>
                  <a href={it.url} target="_blank" rel="noreferrer">
                    {it.name}
                  </a>
                </List.Item>
              )}
            />
            <Divider style={{ margin: '8px 0' }} />
          </div>
        ))}
      </Card>

      <Card title="常见问题">
        <Typography.Paragraph>
          <b>Q: 订阅更新失败？</b><br />
          A: 链接需要外网访问；首次使用可手动复制节点信息粘贴一次；或换用 Clash 格式重试。
        </Typography.Paragraph>
        <Typography.Paragraph>
          <b>Q: 流量跑得很慢？</b><br />
          A: 优先尝试更换节点；高峰期 BGP 拥塞属常态；同一条线路可关闭客户端 TLS 分流再试。
        </Typography.Paragraph>
        <Typography.Paragraph>
          <b>Q: 设备数限制？</b><br />
          A: 默认 3 台，超出后旧设备会被踢下线。可在「套餐」页升级套餐解锁更多。
        </Typography.Paragraph>
      </Card>
    </Space>
  )
}
