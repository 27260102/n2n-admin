export interface Node {
  id: number;
  name: string;
  ip_address: string;
  mac_address: string;
  community: string;
  description: string;
  is_enabled: boolean;
  last_seen?: string;
  created_at: string;
  updated_at: string;
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
}
