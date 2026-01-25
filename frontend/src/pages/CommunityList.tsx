import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, message, Typography } from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { communityApi } from '../api';
import type { Community } from '../types';

const { Title } = Typography;

const CommunityList: React.FC = () => {
  const [communities, setCommunities] = useState<Community[]>([]);
  const [loading, setLoading] = useState(false);
  const [isModalVisible, setIsModalVisible] = useState(false);
  const [form] = Form.useForm();

  const fetchData = async () => {
    setLoading(true);
    try {
      const { data } = await communityApi.list();
      setCommunities(data);
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
      await communityApi.create(values);
      message.success('社区创建成功');
      setIsModalVisible(false);
      form.resetFields();
      fetchData();
    } catch (error) {
      message.error('创建失败');
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await communityApi.delete(id);
      message.success('社区已删除');
      fetchData();
    } catch (error) {
      message.error('删除失败');
    }
  };

  const columns = [
    { title: '社区名称', dataIndex: 'name', key: 'name' },
    { title: 'IP 范围 (CIDR)', dataIndex: 'range', key: 'range' },
    { title: '访问密码', dataIndex: 'password', key: 'password' },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: Community) => (
        <Button 
          icon={<DeleteOutlined />} 
          type="link" 
          danger 
          onClick={() => handleDelete(record.id)}
        >
          删除
        </Button>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Title level={2}>社区设置</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalVisible(true)}>
          新建社区
        </Button>
      </div>

      <Table 
        columns={columns} 
        dataSource={communities} 
        rowKey="id" 
        loading={loading} 
      />

      <Modal
        title="创建新社区"
        open={isModalVisible}
        onOk={() => form.submit()}
        onCancel={() => setIsModalVisible(false)}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="name" label="社区名称" rules={[{ required: true }]}>
            <Input placeholder="例如: office" />
          </Form.Item>
          <Form.Item name="range" label="IP 网段 (CIDR)" rules={[{ required: true }]}>
            <Input placeholder="例如: 10.10.10.0/24" />
          </Form.Item>
          <Form.Item name="password" label="社区密码" rules={[{ required: true }]}>
            <Input.Password placeholder="用于 edge 认证的密码" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default CommunityList;