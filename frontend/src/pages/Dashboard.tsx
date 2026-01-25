import React, { useState, useEffect, useRef } from 'react';
import { Row, Col, Card, Statistic, Typography, Spin, Table, Tag } from 'antd';
import { ClusterOutlined, SafetyCertificateOutlined, GlobalOutlined, SwapOutlined } from '@ant-design/icons';
import { systemApi } from '../api';
import { Network } from 'vis-network';
import type { Node as VisNode, Edge as VisEdge, Options } from 'vis-network';
import { DataSet } from 'vis-data';
import dayjs from 'dayjs';

const { Title, Text } = Typography;

const Dashboard: React.FC = () => {
  const [stats, setStats] = useState<any>(null);
  const [relays, setRelays] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [topoLoading, setTopoLoading] = useState(true);
  const visJsRef = useRef<HTMLDivElement>(null);
  const networkRef = useRef<Network | null>(null);

  const fetchStatsAndRelays = async () => {
    try {
      const [statsRes, relayRes] = await Promise.all([
        systemApi.getStats(),
        systemApi.getRelays()
      ]);
      setStats(statsRes.data);
      setRelays(relayRes.data);
    } catch (error) {
      console.error('Failed to fetch data');
    } finally {
      setLoading(false);
    }
  };

  const fetchTopology = async () => {
    setTopoLoading(true);
    try {
      const { data } = await systemApi.getTopology();
      if (!visJsRef.current) {
        // 如果容器还没准备好，稍后再试一次
        setTimeout(fetchTopology, 100);
        return;
      }

      const nodeData: VisNode[] = data.nodes.map((n: any) => ({
        id: n.id,
        label: n.label,
        shape: n.group === 'supernode' ? 'diamond' : 'dot',
        size: n.group === 'supernode' ? 30 : 20,
        color: n.group === 'supernode' ? '#1677ff' : (n.group === 'online' ? '#52c41a' : '#bfbfbf')
      }));

      const edgeData: VisEdge[] = data.edges.map((e: any) => ({
        from: e.from,
        to: e.to
      }));

      const nodes = new DataSet<VisNode>(nodeData);
      const edges = new DataSet<VisEdge>(edgeData);

      const options: Options = {
        nodes: { font: { size: 12, color: '#333' }, borderWidth: 2, shadow: true },
        edges: { width: 2, color: { inherit: 'from' }, smooth: { enabled: true, type: 'continuous', roundness: 0.5 } },
        physics: { enabled: true, stabilization: { iterations: 50 } },
        interaction: { hover: true, dragNodes: true }
      };

      if (networkRef.current) {
        networkRef.current.setData({ nodes, edges });
      } else {
        networkRef.current = new Network(visJsRef.current, { nodes, edges }, options);
      }
    } catch (error) {
      console.error('Failed to fetch topology');
    } finally {
      setTopoLoading(false);
    }
  };

  useEffect(() => {
    fetchStatsAndRelays();
    fetchTopology();
    
    const timer = setInterval(() => {
      fetchStatsAndRelays();
      fetchTopology();
    }, 5000);
    
    return () => clearInterval(timer);
  }, []);

  const relayColumns = [
    { 
      title: '源节点 (MAC)', 
      dataIndex: 'src_mac', 
      render: (m: string) => <Text code>{m}</Text>
    },
    { 
      title: ' ', 
      render: () => <SwapOutlined style={{ color: '#fa8c16' }} /> 
    },
    { 
      title: '目标节点 (MAC)', 
      dataIndex: 'dst_mac', 
      render: (m: string) => <Text code>{m}</Text>
    },
    { 
      title: '转发包数', 
      dataIndex: 'pkt_count', 
      render: (c: number) => <Tag color="orange">{c}</Tag>
    },
    { 
      title: '活动时间', 
      dataIndex: 'last_active', 
      render: (t: string) => dayjs(t).format('HH:mm:ss')
    },
  ];

  return (
    <div>
      <Title level={2}>网络状态分析</Title>
      
      <Row gutter={16}>
        <Col span={6}>
          <Card hoverable>
            <Statistic title="在线节点" value={stats?.online_count} prefix={<GlobalOutlined style={{ color: '#52c41a' }} />} loading={loading} />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable>
            <Statistic title="已登记节点" value={stats?.node_count} prefix={<ClusterOutlined style={{ color: '#1677ff' }} />} loading={loading} />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable>
            <Statistic title="虚拟社区" value={stats?.community_count} prefix={<SafetyCertificateOutlined style={{ color: '#722ed1' }} />} loading={loading} />
          </Card>
        </Col>
        <Col span={6}>
          <Card hoverable>
            <Statistic title="当前中转对" value={relays.length} prefix={<SwapOutlined style={{ color: '#fa8c16' }} />} valueStyle={{ color: '#fa8c16' }} loading={loading} />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginTop: 24 }}>
        <Col span={14}>
          <Card 
            title="网络拓扑结构" 
            bordered={false} 
            bodyStyle={{ padding: 0, position: 'relative' }}
          >
            {topoLoading && !networkRef.current && (
              <div style={{ position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', zIndex: 10 }}>
                <Spin tip="加载拓扑中..." />
              </div>
            )}
            <div ref={visJsRef} style={{ height: '500px', width: '100%', background: '#fcfcfc' }} />
          </Card>
        </Col>
        <Col span={10}>
          <Card title="实时转发流量 (Relay Activity)" bordered={false}>
            <Table 
              dataSource={relays} 
              columns={relayColumns} 
              size="small" 
              pagination={false} 
              rowKey={(record) => record.src_mac + record.dst_mac}
              loading={loading}
              locale={{ emptyText: '当前没有中转流量' }}
            />
            <div style={{ marginTop: 10 }}>
              <Text type="secondary" style={{ fontSize: '12px' }}>注：如果此处为空，说明节点间正在通过 P2P 直接通信。</Text>
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
