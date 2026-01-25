import React, { useState } from 'react';
import { Layout, Menu, theme, Button, Modal, message, Space, Typography, Dropdown, Form, Input } from 'antd';
import {
  DashboardOutlined,
  ClusterOutlined,
  SettingOutlined,
  SafetyCertificateOutlined,
  LogoutOutlined,
  UserOutlined,
  KeyOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';
import axios from 'axios';

const { Header, Content, Sider } = Layout;
const { Text } = Typography;

interface MainLayoutProps {
  children: React.ReactNode;
}

const MainLayout: React.FC<MainLayoutProps> = ({ children }) => {
  const navigate = useNavigate();
  const location = useLocation();
  const [isPwdModalOpen, setIsPwdModalOpen] = useState(false);
  const [pwdLoading, setPwdLoading] = useState(false);
  const [pwdForm] = Form.useForm();
  
  const {
    token: { colorBgContainer, borderRadiusLG },
  } = theme.useToken();

  const userJson = localStorage.getItem('n2n_user');
  const user = userJson ? JSON.parse(userJson) : { username: 'Admin' };

  const handleLogout = () => {
    Modal.confirm({
      title: '确认退出',
      content: '您确定要退出当前管理面板吗？',
      onOk: () => {
        localStorage.removeItem('n2n_token');
        localStorage.removeItem('n2n_user');
        message.success('已退出登录');
        navigate('/login');
      },
    });
  };

  const handleChangePassword = async (values: any) => {
    setPwdLoading(true);
    try {
      await axios.post('/api/change-password', values, {
        headers: { Authorization: `Bearer ${localStorage.getItem('n2n_token')}` }
      });
      message.success('密码修改成功，请重新登录');
      setIsPwdModalOpen(false);
      localStorage.removeItem('n2n_token');
      navigate('/login');
    } catch (error: any) {
      message.error(error.response?.data?.error || '修改失败');
    } finally {
      setPwdLoading(false);
    }
  };

  const userMenuItems = [
    {
      key: 'pwd',
      label: '修改密码',
      icon: <KeyOutlined />,
      onClick: () => setIsPwdModalOpen(true),
    },
    {
      type: 'divider',
    },
    {
      key: 'logout',
      label: '退出登录',
      icon: <LogoutOutlined />,
      danger: true,
      onClick: handleLogout,
    },
  ];

  const menuItems = [
    { key: '/', icon: <DashboardOutlined />, label: '仪表盘' },
    { key: '/nodes', icon: <ClusterOutlined />, label: '节点管理' },
    { key: '/communities', icon: <SafetyCertificateOutlined />, label: '社区设置' },
    { key: '/settings', icon: <SettingOutlined />, label: '系统设置' },
  ];

  return (
    <Layout style={{ minHeight: '100vh', width: '100%' }}>
      <Sider breakpoint="lg" collapsedWidth="0" theme="light" style={{ borderRight: '1px solid #f0f0f0' }}>
        <div style={{ height: 64, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '18px', fontWeight: 'bold', color: '#1677ff', borderBottom: '1px solid #f0f0f0', marginBottom: 16 }}>
          n2n Admin
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100% - 80px)', justifyContent: 'space-between' }}>
          <Menu
            mode="inline"
            selectedKeys={[location.pathname]}
            items={menuItems}
            onClick={({ key }) => navigate(key)}
            style={{ borderRight: 0 }}
          />
          <div style={{ padding: '16px', borderTop: '1px solid #f0f0f0' }}>
            <Button type="text" danger icon={<LogoutOutlined />} onClick={handleLogout} block style={{ textAlign: 'left', height: '40px' }}>
              退出登录
            </Button>
          </div>
        </div>
      </Sider>
      <Layout>
        <Header style={{ padding: '0 24px', background: colorBgContainer, display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid #f0f0f0' }}>
          <h3 style={{ margin: 0 }}>n2n 网络管理系统</h3>
          <Dropdown menu={{ items: userMenuItems as any }} placement="bottomRight">
            <Space style={{ cursor: 'pointer' }}>
              <UserOutlined />
              <Text strong>{user.username}</Text>
            </Space>
          </Dropdown>
        </Header>
        <Content style={{ margin: '24px', minHeight: 'initial' }}>
          <div style={{ padding: 24, background: colorBgContainer, borderRadius: borderRadiusLG, minHeight: 'calc(100vh - 112px)' }}>
            {children}
          </div>
        </Content>
      </Layout>

      <Modal
        title="修改管理员密码"
        open={isPwdModalOpen}
        onCancel={() => setIsPwdModalOpen(false)}
        onOk={() => pwdForm.submit()}
        confirmLoading={pwdLoading}
      >
        <Form form={pwdForm} layout="vertical" onFinish={handleChangePassword}>
          <Form.Item name="old_password" label="当前密码" rules={[{ required: true }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="new_password" label="新密码" rules={[{ required: true, min: 6 }]}>
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>
    </Layout>
  );
};

export default MainLayout;
