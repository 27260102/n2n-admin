export interface Node {
  id: number;
  name: string;
  ip_address: string;
  mac_address: string;
  community: string;
  description: string;
  encryption?: string;
  compression?: boolean;
  routing?: string;
  local_port?: number;
  is_enabled: boolean;
  last_seen?: string;
  created_at: string;
  updated_at: string;
  // 运行时字段（后端返回）
  is_online?: boolean;
  is_mapped?: boolean;
  external_ip?: string;
  location?: string;
  conn_type?: 'P2P' | 'Relay';
}

export interface Community {
  id: number;
  name: string;
  range: string;
  password?: string;
  created_at: string;
}

export interface ConfigParams {
  name: string;
  ip: string;
  community: string;
  password?: string;
  supernode: string;
  mac: string;
  encryption?: string;
  compression?: boolean;
  routing?: string;
  local_port?: number;
}

export interface RelayEvent {
  src_mac: string;
  dst_mac: string;
  last_active: string;
  pkt_count: number;
}

export interface Stats {
  node_count: number;
  community_count: number;
  online_count: number;
}

export interface TopologyNode {
  id: string;
  label: string;
  group: 'supernode' | 'online' | 'offline';
}

export interface TopologyEdge {
  from: string;
  to: string;
}

export interface TopologyData {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
}

export interface Settings {
  supernode_host?: string;
  [key: string]: string | undefined;
}

export interface SnConfig {
  p?: string;
  t?: string;
  c?: string;
  [key: string]: string | undefined;
}

export interface NodeFormValues {
  name: string;
  mac_address?: string;
  community: string;
  ip_address?: string;
  encryption?: string;
  compression?: boolean;
  route_net?: string;
  route_gw?: string;
}

export interface CommunityFormValues {
  name: string;
  range: string;
  password: string;
}

export interface ApiError {
  error: string;
  message?: string;
}

export interface HealthResponse {
  status: string;
  version: string;
}

export interface LogsResponse {
  logs: string[];
}
