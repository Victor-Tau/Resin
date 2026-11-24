import { apiRequest } from "../../lib/api-client";
import { listEndpoints } from "../endpoints/api";
import type { Endpoint, SOCKS5Account } from "../endpoints/types";

export type IPv6ConfigInput = SOCKS5Account & { interface: string; endpoint_ids: string[]; prefix_id?: string };
export type IPv6Config = IPv6ConfigInput & { id: string };
export type IPv6Candidate = { prefix: string; parent: string };
export type IPv6Interface = { name: string; prefixes: string[]; candidates?: IPv6Candidate[] };
export type IPv6PrefixInput = { label: string; interface: string; prefix: string };
export type IPv6Prefix = IPv6PrefixInput & { id: string; proxy_count: number };
export type IPv6PrefixCheckResult = {
  address: string;
  assigned: boolean;
  egress: boolean;
  stage?: "assign" | "egress" | "cleanup";
  error?: string;
};

const basePath = "/api/v1/ipv6-configs";
const prefixesPath = "/api/v1/ipv6-prefixes";

export function listIPv6Prefixes() {
  return apiRequest<IPv6Prefix[]>(prefixesPath);
}

export function saveIPv6Prefix(input: IPv6PrefixInput, id?: string) {
  return id
    ? apiRequest<IPv6Prefix>(`${prefixesPath}/${encodeURIComponent(id)}`, { method: "PUT", body: input })
    : apiRequest<IPv6Prefix>(prefixesPath, { method: "POST", body: input });
}

export function deleteIPv6Prefix(id: string) {
  return apiRequest<void>(`${prefixesPath}/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function checkIPv6Prefix(input: Pick<IPv6PrefixInput, "interface" | "prefix">) {
  return apiRequest<IPv6PrefixCheckResult>(`${prefixesPath}/check`, { method: "POST", body: input });
}

export function listIPv6Interfaces() {
  return apiRequest<IPv6Interface[]>("/api/v1/system/ipv6-interfaces");
}

export async function listIPv6Endpoints() {
  const endpoints: Endpoint[] = [];
  for (let offset = 0; ; offset = endpoints.length) {
    const page = await listEndpoints({ limit: 100, offset, includeIPv6Passwords: true });
    endpoints.push(...page.items);
    if (endpoints.length >= page.total || page.items.length === 0) {
      return endpoints;
    }
  }
}

export function listIPv6Configs() {
  return apiRequest<IPv6Config[]>(basePath);
}

export function createIPv6Configs(items: IPv6ConfigInput[]) {
  return apiRequest<IPv6Config[]>(basePath, { method: "POST", body: { items } });
}

export function updateIPv6Config(id: string, input: IPv6ConfigInput) {
  return apiRequest<IPv6Config>(`${basePath}/${encodeURIComponent(id)}`, { method: "PUT", body: input });
}

export function deleteIPv6Config(id: string) {
  return apiRequest<void>(`${basePath}/${encodeURIComponent(id)}`, { method: "DELETE" });
}
