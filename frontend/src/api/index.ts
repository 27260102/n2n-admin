import axios from 'axios';
import type { Node, Community } from '../types';

const api = axios.create({
  baseURL: '/api',
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
    if (!window.location.pathname.startsWith('/login')) window.location.href = '/login';
  }
  return Promise.reject(error);
});

export const nodeApi = {
  list: () => api.get<Node[]>('/nodes'),
  create: (data: Partial<Node>) => api.post<Node>('/nodes', data),
  delete: (id: number) => api.delete(`/nodes/${id}`),
  getConfig: (id: number) => api.get(`/nodes/${id}/config`),
};

export const communityApi = {
  list: () => api.get<Community[]>('/communities'),
  create: (data: Partial<Community>) => api.post<Community>('/communities', data),
  delete: (id: number) => api.delete(`/communities/${id}`),
};

export const systemApi = {
  getStats: () => api.get('/stats'),
  getTopology: () => api.get('/topology'),
  getSettings: () => api.get('/settings'),
  saveSettings: (data: any) => api.post('/settings', data),
  getSnConfig: () => api.get('/supernode/config'),
  saveSnConfig: (data: any) => api.post('/supernode/config', data),
  restartSn: () => api.post('/supernode/restart'),
  execTool: (command: string, target: string) => api.post('/tools/exec', { command, target }),
  getRelays: () => api.get('/relays'), // 新增
};

export default api;