import React, { useState, useEffect } from 'react';
import { Table, Button, Space, Modal, Form, Input, message, Tag, Typography, Select, Switch, Row, Col, Divider, Radio, Tooltip } from 'antd';
import { PlusOutlined, DownloadOutlined, DeleteOutlined, ToolOutlined, GlobalOutlined, HomeOutlined } from '@ant-design/icons';
import { nodeApi, communityApi, systemApi } from '../api';
import type { Node, Community } from '../types';

const { Text } = Typography;
const { Option } = Select;

const NodeList: React.FC = () => {
  const [nodes, setNodes] = useState<any[]>([]);
  const [communities, setCommunities] = useState<Community[]>([]);
  const [loading, setLoading] = useState(false);
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [isConfigModalVisible, setIsConfigModalVisible] = useState(false);
  const [isToolModalVisible, setIsToolModalVisible] = useState(false);
  const [toolLoading, setToolLoading] = useState(false);
  const [toolResult, setToolResult] = useState('');
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);
  const [toolCommand, setToolCommand] = useState('ping');
  
  const [currentConfig, setCurrentConfig] = useState<any>(null);
  const [form] = Form.useForm();

  const fetchData = async () => {
    setLoading(true);
    try {
      const [nodesRes, commRes] = await Promise.all([
        nodeApi.list(),
        communityApi.list()
      ]);
      setNodes(nodesRes.data);
      setCommunities(commRes.data);
    } catch (error) {
      message.error('数据加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleCreate = async (values: any) => {
    try {
      await nodeApi.create(values);
      message.success('节点保存成功');
      setIsModalVisible(false);
      form.resetFields();
      fetchData();
    } catch (error) {
      message.error('保存失败');
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await nodeApi.delete(id);
      message.success('节点已删除');
      fetchData();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const showConfig = async (id: number) => {
    try {
      const { data } = await nodeApi.getConfig(id);
      setCurrentConfig({ ...data, id });
      setIsConfigModalVisible(true);
    } catch (error) {
      message.error('获取配置失败');
    }
  };

  const showTool = (node: Node) => {
    setSelectedNode(node);
    setToolResult('');
    setIsToolModalVisible(true);
  };

  const handleExecTool = async () => {
    if (!selectedNode) return;
    setToolLoading(true);
    setToolResult('正在执行，请稍候...\n');
    try {
      const { data } = await systemApi.execTool(toolCommand, selectedNode.ip_address);
      setToolResult(data.output);
    } catch (error: any) {
      setToolResult(error.response?.data?.output || error.response?.data?.error || '执行出错');
    } finally {
      setToolLoading(false);
    }
  };

  const handleDownload = () => {
    if (!currentConfig) return;
    const blob = new Blob([currentConfig.conf], { type: 'text/plain' });
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `edge_${currentConfig.id}.conf`;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);
    message.success('配置文件已准备下载');
  };

  const columns = [
    {
      title: '节点信息',
      dataIndex: 'name',
      key: 'name',
      render: (text: string, record: any) => (
        <div>
          <div style={{ fontWeight: 'bold' }}>
            {text} {!record.is_mapped && <Tag color="warning">未登记</Tag>}
          </div>
          <div style={{ fontSize: '12px', color: '#999' }}>{record.mac_address}</div>
        </div>
      ),
    },
    {
      title: '虚拟 IP',
      dataIndex: 'ip_address',
      key: 'ip_address',
      render: (ip: string) => (
        <Text copyable style={{ fontFamily: 'monospace' }}>
          <Tag color="blue" style={{ fontFamily: 'monospace', marginRight: 0 }}>{ip}</Tag>
        </Text>
      ),
    },
    {
      title: '外网出口 (GeoIP)',
      key: 'public_info',
      render: (_: any, record: any) => (
        record.is_online ? (
          <div>
            <div><GlobalOutlined style={{ color: '#1890ff', marginRight: 5 }} /><Text copyable>{record.external_ip}</Text></div>
            <div style={{ fontSize: '12px', color: '#8c8c8c' }}><HomeOutlined style={{ marginRight: 5 }} />{record.location}</div>
          </div>
        ) : <Text type="secondary">-</Text>
      ),
    },
    {
      title: '社区',
      dataIndex: 'community',
      key: 'community',
    },
    {
      title: '状态',
      key: 'status',
      render: (_: any, record: any) => (
        <Space>
          <Tag color={record.is_online ? 'green' : 'default'}>
            {record.is_online ? '在线' : '离线'}
          </Tag>
        </Space>
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Space size="small">
          {record.is_mapped ? (
            <>
              <Tooltip title="网络测试工具"><Button icon={<ToolOutlined />} size="small" onClick={() => showTool(record)} /></Tooltip>
              <Button icon={<DownloadOutlined />} type="link" onClick={() => showConfig(record.id)}>配置</Button>
              <Button icon={<DeleteOutlined />} type="link" danger onClick={() => handleDelete(record.id)}>删除</Button>
            </>
          ) : (
            <Button type="primary" size="small" onClick={() => {
              form.setFieldsValue({ name: '新节点', mac_address: record.mac_address });
              setIsModalVisible(true);
            }}>加入管理</Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography.Title level={2}>节点管理</Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => {
          form.resetFields();
          setIsModalVisible(true);
        }}>
          新建节点
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={nodes}
        rowKey={(record) => record.id || record.mac_address}
        loading={loading}
        pagination={{
          defaultPageSize: 20,
          showSizeChanger: true,
          pageSizeOptions: ['10', '20', '50', '100'],
          showTotal: (total) => `共 ${total} 个节点`
        }}
        scroll={{ x: 1000 }}
      />

      {/* 登记节点 Modal */}
      <Modal
        title="登记 n2n 节点"
        open={isModalVisible}
        onOk={() => form.submit()}
        onCancel={() => setIsModalVisible(false)}
        width={700}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate} initialValues={{ encryption: 'AES', compression: false }}>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label="节点名称 (必填)" rules={[{ required: true }]}>
                <Input placeholder="例如: MyLaptop" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item 
                name="mac_address" 
                label="MAC 地址 (留空则自动生成)"
                rules={[{ pattern: /^([0-9A-Fa-f]{2}[:-]?){5}([0-9A-Fa-f]{2})$/, message: 'MAC 格式不正确' }]}>
                <Input placeholder="例如: AA:BB:CC:DD:EE:FF" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="community" label="选择社区" rules={[{ required: true }]}>
                <Select placeholder="请选择所属社区">
                  {communities.map(c => (
                    <Option key={c.name} value={c.name}>{c.name} ({c.range})</Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item 
                name="ip_address" 
                label="指定虚拟 IP (可选)"
                rules={[{ pattern: /^([0-9]{1,3}\.){3}[0-9]{1,3}$/, message: 'IP 格式不正确' }]}>
                <Input placeholder="留空则自动分配" />
              </Form.Item>
            </Col>
          </Row>

          <Divider plain>高级路由</Divider>
          
          <Row gutter={16}>
            <Col span={10}>
              <Form.Item name="route_net" label="目标局域网段"><Input placeholder="192.168.1.0/24" /></Form.Item>
            </Col>
            <Col span={10}>
              <Form.Item name="route_gw" label="中转网关 (虚拟 IP)"><Input placeholder="10.10.10.1" /></Form.Item>
            </Col>
            <Col span={4}>
              <Form.Item name="compression" label="压缩" valuePropName="checked"><Switch /></Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      {/* 配置预览 Modal */}
      <Modal
        title="edge.conf 配置文件"
        open={isConfigModalVisible}
        onCancel={() => setIsConfigModalVisible(false)}
        footer={[
          <Button key="download" type="primary" icon={<DownloadOutlined />} onClick={handleDownload}>下载 .conf</Button>,
          <Button key="close" onClick={() => setIsConfigModalVisible(false)}>关闭</Button>
        ]}
      >
        {currentConfig && (
          <div style={{ marginTop: 16 }}>
            <Typography.Paragraph>请保存到 <code>/etc/n2n/edge.conf</code> 并重启服务。</Typography.Paragraph>
            <Typography.Paragraph copyable={{ text: currentConfig.conf }}>
              <pre style={{ background: '#f5f5f5', padding: '15px', borderRadius: '4px' }}>{currentConfig.conf}</pre>
            </Typography.Paragraph>
          </div>
        )}
      </Modal>

      {/* 工具箱 Modal */}
      <Modal
        title={`网络测试工具 - ${selectedNode?.name}`}
        open={isToolModalVisible}
        onCancel={() => setIsToolModalVisible(false)}
        width={650}
        footer={[<Button key="close" onClick={() => setIsToolModalVisible(false)}>关闭</Button>]}
      >
        <div style={{ marginBottom: 20 }}>
          <Radio.Group value={toolCommand} onChange={(e) => setToolCommand(e.target.value)}>
            <Radio.Button value="ping">Ping</Radio.Button>
            <Radio.Button value="traceroute">Traceroute</Radio.Button>
          </Radio.Group>
            
          <Button type="primary" style={{ marginLeft: 16 }} onClick={handleExecTool} loading={toolLoading}>执行</Button>
        </div>
        <div style={{ background: '#001529', color: '#00ff00', padding: '15px', borderRadius: '4px', fontFamily: 'monospace', minHeight: '200px', whiteSpace: 'pre-wrap' }}>
          {toolResult || '等待执行...'}
        </div>
      </Modal>
    </div>
  );
};

export default NodeList;
