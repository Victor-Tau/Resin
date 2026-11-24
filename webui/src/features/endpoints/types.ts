export type EndpointStatus = "active" | "starting" | "inactive" | "error" | string;

export type SOCKS5Account = { username: string; password?: string; address: string };

export type Endpoint = {
  id: string;
  port: number;
  enabled: boolean;
  allow_management: boolean;
  allow_proxy: boolean;
  require_proxy_auth_info: boolean;
  allow_http_forward: boolean;
  allow_http_reverse: boolean;
  allow_socks5: boolean;
  socks5_ipv6_interface: string;
  socks5_accounts: SOCKS5Account[];
  source: "environment" | "database" | string;
  read_only: boolean;
  status: EndpointStatus;
  last_error?: string;
  created_at?: string;
  updated_at?: string;
};

export type EndpointInput = {
  port: number;
  enabled: boolean;
  allow_management: boolean;
  allow_proxy: boolean;
  require_proxy_auth_info: boolean;
  allow_http_forward: boolean;
  allow_http_reverse: boolean;
  allow_socks5: boolean;
};

export type EndpointPatch = Partial<EndpointInput & Pick<Endpoint, "socks5_ipv6_interface" | "socks5_accounts">>;

export type EndpointListResponse = {
  items: Endpoint[];
  total: number;
  limit: number;
  offset: number;
};
