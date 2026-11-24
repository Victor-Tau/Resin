import type { Endpoint } from "../endpoints/types";
import type { IPv6Config } from "./api";

const passwordKey = "resin_ipv6_passwords";

export function loadIPv6Passwords(): Map<string, string> {
  try {
    return new Map(JSON.parse(sessionStorage.getItem(passwordKey) ?? "[]"));
  } catch {
    return new Map();
  }
}

export function storeIPv6Passwords(passwords: Map<string, string>) {
  try {
    if (passwords.size) {
      sessionStorage.setItem(passwordKey, JSON.stringify(Array.from(passwords)));
    } else {
      sessionStorage.removeItem(passwordKey);
    }
  } catch {
    // The in-memory copy still works when browser storage is disabled.
  }
}

export function socks5Ports(config: IPv6Config, endpoints: Endpoint[]) {
  return endpoints
    .filter((endpoint) => config.endpoint_ids.includes(endpoint.id) && endpoint.enabled && endpoint.allow_proxy && endpoint.allow_socks5)
    .map((endpoint) => endpoint.port)
    .sort((a, b) => a - b);
}

export function socks5Links(config: IPv6Config, password: string, ports: number[]) {
  const hostname = window.location.hostname;
  const host = hostname.includes(":") && !hostname.startsWith("[") ? `[${hostname}]` : hostname;
  const credentials = `${encodeURIComponent(config.username)}:${encodeURIComponent(password)}`;
  return ports.map((port) => `socks5h://${credentials}@${host}:${port}`);
}
