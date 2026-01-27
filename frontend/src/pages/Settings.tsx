import React, { useState, useEffect, useRef } from 'react';
import { Card, Form, Input, Button, message, Typography, Row, Col, Alert, Space } from 'antd';
import { SaveOutlined, ReloadOutlined, ProfileOutlined } from '@ant-design/icons';
import { systemApi } from '../api';
import axios from 'axios';

const { Title, Text } = Typography;

const Settings: React.FC = () => {
  const [globalForm] = Form.useForm();
  const [snForm] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [snLoading, setSnLoading] = useState(false);
  const [logs, setLogs] = useState<string[]>([]);
  const [version, setVersion] = useState('v1.x.x');
  const logContainerRef = useRef<HTMLDivElement>(null);

  const fetchData = async () => {
    try {
      const [settingsRes, snRes, healthRes] = await Promise.all([
        systemApi.getSettings(),
        systemApi.getSnConfig(),
        axios.get('/api/health')
      ]);
      globalForm.setFieldsValue(settingsRes.data);
      snForm.setFieldsValue(snRes.data);
      setVersion(healthRes.data.version || 'v1.2.2');
    } catch (error) {
      console.error('Failed to fetch settings');
    }
  };

  const fetchLogs = async () => {
    try {
      const { data } = await systemApi.getRecentLogs();
      setLogs(data.logs || []);
    } catch (error) {
      console.error('Failed to fetch logs');
    }
  };

  useEffect(() => {
    fetchData();
    fetchLogs();

    // 使用轮询代替 EventSource，避免 token 暴露在 URL 中
    const logTimer = setInterval(fetchLogs, 3000);

    return () => {
      clearInterval(logTimer);
    };
  }, []);

  useEffect(() => {
    // 只在日志容器内部滚动到底部，不影响页面滚动位置
    if (logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight;
    }
  }, [logs]);

  const onGlobalFinish = async (values: any) => {
    setLoading(true);
    try {
      await systemApi.saveSettings(values);
      message.success('全局设置已保存');
    } catch (error) {
      message.error('保存失败');
    } finally {
      setLoading(false);
    }
  };

  const onSnFinish = async (values: any) => {
    setSnLoading(true);
    try {
      await systemApi.saveSnConfig(values);
      message.success('Supernode 配置已更新');
    } catch (error) {
      message.error('配置保存失败');
    } finally {
      setSnLoading(false);
    }
  };

  const handleRestart = async () => {
    setSnLoading(true);
    try {
      await systemApi.restartSn();
      message.success('Supernode 服务已重启');
      setLogs([]); 
    } catch (error: any) {
      message.error('服务重启失败: ' + (error.response?.data?.error || '未知错误'));
    } finally {
      setSnLoading(false);
    }
  };

  return (
    <div style={{ maxWidth: 1000, paddingBottom: 50 }}>
      <Title level={2}>系统设置</Title>
      
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <Card title="1. 节点默认配置 (模版)" bordered={false}>
          <Alert 
            message="说明" 
            description="此处设置的参数将作为默认值，自动填充到系统生成的 edge.conf 配置文件中。" 
            type="success" 
            showIcon 
            style={{ marginBottom: 20 }}
          />
          <Form form={globalForm} layout="vertical" onFinish={onGlobalFinish}>
            <Form.Item 
              name="supernode_host" 
              label="Supernode 服务地址 (Host:Port)" 
              rules={[{ required: true }]}
              extra="此地址将写入生成的 edge.conf。请确保客户端能通过该地址访问到本服务器。"
            >
              <Input placeholder="1.2.3.4:7654" />
            </Form.Item>
            <Button type="primary" icon={<SaveOutlined />} htmlType="submit" loading={loading}>
              保存模版设置
            </Button>
          </Form>
        </Card>

        <Card title="2. Supernode 服务管理" bordered={false}>
          <Alert 
            message="管理说明" 
            description="修改后需重启服务生效。管理端口建议保持为 56440。" 
            type="info" 
            showIcon 
            style={{ marginBottom: 20 }}
          />
          <Form form={snForm} layout="vertical" onFinish={onSnFinish}>
            <Row gutter={24}>
              <Col span={12}>
                <Form.Item name="p" label="主监听端口 (-p)"><Input /></Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item name="t" label="管理端口 (-t)"><Input /></Form.Item>
              </Col>
            </Row>
            <Form.Item name="c" label="社区文件路径 (-c)"><Input /></Form.Item>
            <Space>
              <Button type="primary" icon={<SaveOutlined />} htmlType="submit" loading={snLoading}>保存配置</Button>
              <Button danger icon={<ReloadOutlined />} onClick={handleRestart} loading={snLoading}>重启服务</Button>
            </Space>
          </Form>
        </Card>

        <Card title={<span><ProfileOutlined /> Supernode 实时日志</span>} bordered={false}>
          <div
            ref={logContainerRef}
            style={{
              background: '#1e1e1e',
              color: '#d4d4d4',
              padding: '15px',
              borderRadius: '4px',
              fontFamily: "'Fira Code', 'Courier New', monospace",
              fontSize: '13px',
              height: '400px',
              overflowY: 'auto',
              whiteSpace: 'pre-wrap'
            }}
          >
            {logs.length > 0 ? (
              logs.map((log, index) => (
                <div key={index} style={{ marginBottom: '2px', borderBottom: '1px solid #333' }}>
                  {log}
                </div>
              ))
            ) : (
              <div style={{ color: '#666' }}>正在连接日志流...</div>
            )}
          </div>
          <div style={{ marginTop: 10, textAlign: 'right' }}>
            <Text type="secondary">仅显示最近 200 条记录</Text>
          </div>
        </Card>

        <Card title="关于系统" bordered={false}>
          <Text type="secondary">n2n Web UI {version}</Text>
        </Card>
      </Space>
    </div>
  );
};

export default Settings;