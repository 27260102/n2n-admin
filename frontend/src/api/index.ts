import axios, { AxiosError } from 'axios';
import { message } from 'antd';
import type {
  Node,
  Community,
  Stats,
  TopologyData,
  RelayEvent,
  Settings,
  SnConfig,
  NodeFormValues,
  CommunityFormValues,
  LogsResponse,
  ApiError
} from '../types';

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('n2n_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
}, (error) => Promise.reject(error));

api.interceptors.response.use((r) => r, (error) => {
  if (error.response?.status === 401) {
    localStorage.removeItem('n2n_token');
    localStorage.removeItem('n2n_user');
    if (!window.location.pathname.startsWith('/login')) {
      window.location.href = '/login';
    }
  }
  return Promise.reject(error);
});

/**
 * 统一错误处理函数
 * @param error - Axios 错误对象
 * @param defaultMessage - 默认错误消息
 * @returns 错误消息字符串
 */
export const handleApiError = (error: unknown, defaultMessage = '操作失败'): string => {
  if (axios.isAxiosError(error)) {
    const axiosError = error as AxiosError<ApiError>;
    const errorMessage =
      axiosError.response?.data?.error ||
      axiosError.response?.data?.message ||
      axiosError.message ||
      defaultMessage;
    return errorMessage;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return defaultMessage;
};

/**
 * 显示 API 错误消息
 */
export const showApiError = (error: unknown, defaultMessage = '操作失败'): void => {
  const msg = handleApiError(error, defaultMessage);
  message.error(msg);
};

export const nodeApi = {
  list: () => api.get<Node[]>('/nodes'),
  create: (data: NodeFormValues) => api.post<Node>('/nodes', data),
  delete: (id: number) => api.delete(`/nodes/${id}`),
  getConfig: (id: number) => api.get<{ conf: string }>(`/nodes/${id}/config`),
};

export const communityApi = {
  list: () => api.get<Community[]>('/communities'),
  create: (data: CommunityFormValues) => api.post<Community>('/communities', data),
  delete: (id: number) => api.delete(`/communities/${id}`),
};

export const systemApi = {
  getStats: () => api.get<Stats>('/stats'),
  getTopology: () => api.get<TopologyData>('/topology'),
  getSettings: () => api.get<Settings>('/settings'),
  saveSettings: (data: Settings) => api.post('/settings', data),
  getSnConfig: () => api.get<SnConfig>('/supernode/config'),
  saveSnConfig: (data: SnConfig) => api.post('/supernode/config', data),
  restartSn: () => api.post('/supernode/restart'),
  execTool: (command: string, target: string) => api.post<{ output: string; error?: string }>('/tools/exec', { command, target }),
  getRelays: () => api.get<RelayEvent[]>('/relays'),
  getRecentLogs: () => api.get<LogsResponse>('/supernode/logs/recent'),
};

export default api;
