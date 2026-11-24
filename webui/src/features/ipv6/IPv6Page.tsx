import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Cable, ChevronRight, KeyRound, Layers, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Card } from "../../components/ui/Card";
import { Textarea } from "../../components/ui/Textarea";
import { ToastContainer } from "../../components/ui/Toast";
import { useToast } from "../../hooks/useToast";
import { useI18n } from "../../i18n";
import { formatApiErrorMessage } from "../../lib/error-message";
import { getEndpoint, updateEndpoint } from "../endpoints/api";
import type { SOCKS5Account } from "../endpoints/types";
import {
  createIPv6Configs,
  deleteIPv6Config,
  listIPv6Configs,
  listIPv6Endpoints,
  listIPv6Interfaces,
  listIPv6Prefixes,
  updateIPv6Config,
  type IPv6Config,
  type IPv6ConfigInput,
  type IPv6Prefix,
} from "./api";
import { IPv6Editor } from "./IPv6Editor";
import { IPv6Prefixes } from "./IPv6Prefixes";

type Entry = {
  config: IPv6Config;
  legacyEndpointID?: string;
};

async function changeLegacyAccount(endpointID: string, username: string, replacement?: SOCKS5Account) {
  const endpoint = await getEndpoint(endpointID);
  if (!endpoint.socks5_accounts.some((account) => account.username === username)) {
    throw new Error("IPv6 配置已不存在");
  }
  const accounts = endpoint.socks5_accounts.flatMap((account) =>
    account.username === username ? (replacement ? [replacement] : []) : [account],
  );
  await updateEndpoint(endpointID, {
    socks5_ipv6_interface: accounts.length ? endpoint.socks5_ipv6_interface : "",
    socks5_accounts: accounts,
  });
}

export function IPv6Page() {
  const { t } = useI18n();
  const client = useQueryClient();
  const { toasts, showToast, dismissToast } = useToast();
  const [editor, setEditor] = useState<Entry | null>();
  const [defaultPrefix, setDefaultPrefix] = useState<IPv6Prefix>();
  const [credentials, setCredentials] = useState<IPv6ConfigInput[]>([]);
  const configsQuery = useQuery({ queryKey: ["ipv6-configs"], queryFn: listIPv6Configs });
  const endpointsQuery = useQuery({ queryKey: ["endpoints", "ipv6"], queryFn: listIPv6Endpoints });
  const interfacesQuery = useQuery({ queryKey: ["ipv6-interfaces"], queryFn: listIPv6Interfaces });
  const prefixesQuery = useQuery({ queryKey: ["ipv6-prefixes"], queryFn: listIPv6Prefixes });
  const endpoints = endpointsQuery.data ?? [];
  const prefixes = prefixesQuery.data ?? [];

  const entries: Entry[] = (configsQuery.data ?? []).map((config) => ({ config }));
  for (const endpoint of endpoints) {
    for (const account of endpoint.socks5_accounts) {
      entries.push({
        config: {
          id: `legacy:${endpoint.id}:${account.username}`,
          username: account.username,
          address: account.address,
          interface: endpoint.socks5_ipv6_interface,
          endpoint_ids: [endpoint.id],
        },
        legacyEndpointID: endpoint.id,
      });
    }
  }

  const invalidate = () => Promise.all([
    client.invalidateQueries({ queryKey: ["ipv6-configs"] }),
    client.invalidateQueries({ queryKey: ["ipv6-prefixes"] }),
    client.invalidateQueries({ queryKey: ["endpoints"] }),
  ]);

  const save = useMutation({
    mutationFn: async ({ entry, items }: { entry: Entry | null; items: IPv6ConfigInput[] }) => {
      if (entry?.legacyEndpointID) {
        const { username, password, address } = items[0];
        await changeLegacyAccount(entry.legacyEndpointID, entry.config.username, { username, password, address });
      } else if (entry) {
        await updateIPv6Config(entry.config.id, items[0]);
      } else {
        await createIPv6Configs(items);
      }
    },
    onSuccess: async (_, { items }) => {
      setCredentials(items.filter((item) => item.password));
      setEditor(undefined);
      await invalidate();
      showToast("success", t("IPv6 配置已保存"));
    },
    onError: (error) => showToast("error", formatApiErrorMessage(error, t)),
  });

  const remove = useMutation({
    mutationFn: async ({ config, legacyEndpointID }: Entry) => {
      if (legacyEndpointID) {
        await changeLegacyAccount(legacyEndpointID, config.username);
      } else {
        await deleteIPv6Config(config.id);
      }
    },
    onSuccess: async () => {
      await invalidate();
      showToast("success", t("IPv6 配置已删除"));
    },
    onError: (error) => showToast("error", formatApiErrorMessage(error, t)),
  });

  const loading = configsQuery.isPending || endpointsQuery.isPending;
  const pending = save.isPending || remove.isPending;
  const error = configsQuery.error ?? endpointsQuery.error ?? prefixesQuery.error ?? interfacesQuery.error;
  const portByID = new Map(endpoints.map((endpoint) => [endpoint.id, endpoint.port]));
  const prefixByID = new Map(prefixes.map((prefix) => [prefix.id, prefix]));
  const proxyFormUnavailable = pending || endpointsQuery.isPending || prefixesQuery.isPending;

  function openEditor(entry: Entry | null, prefix?: IPv6Prefix) {
    setDefaultPrefix(prefix);
    setEditor(entry);
    requestAnimationFrame(() => document.getElementById("ipv6-proxy-editor")?.scrollIntoView({ block: "nearest" }));
  }

  const flow = [
    { icon: Layers, label: "号段", description: "决定代理的出口地址" },
    { icon: KeyRound, label: "代理账号", description: "用账号和密码登录" },
    { icon: Cable, label: "连接端口", description: "客户端通过端口接入" },
  ];

  return (
    <section className="platform-page">
      <header className="module-header">
        <div>
          <h2>{t("IPv6 管理")}</h2>
          <p className="module-description">{t("为 SOCKS5 代理分配固定的 IPv6 出口地址。")}</p>
        </div>
        <Button onClick={() => openEditor(null)} disabled={proxyFormUnavailable}>
          <Plus size={16} />
          {t("创建 IPv6 代理")}
        </Button>
      </header>
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />

      {error && <div className="callout callout-error" role="alert">{formatApiErrorMessage(error, t)}</div>}
      <ol className="ipv6-flow" aria-label={t("IPv6 代理配置流程")}>
        {flow.map(({ icon: Icon, label, description }, index) => (
          <li key={label}>
            <span className="ipv6-icon-tile"><Icon size={19} aria-hidden="true" /></span>
            <div>
              <strong>{t(label)}</strong>
              <p className="muted">{t(description)}</p>
            </div>
            {index < flow.length - 1 && <ChevronRight className="ipv6-flow-arrow" size={17} aria-hidden="true" />}
          </li>
        ))}
      </ol>
      <IPv6Prefixes
        prefixes={prefixes}
        interfaces={interfacesQuery.data ?? []}
        loading={prefixesQuery.isPending}
        detecting={interfacesQuery.isFetching}
        onDetect={() => void interfacesQuery.refetch()}
        proxyPending={pending}
        canCreateProxy={!proxyFormUnavailable}
        onUse={(prefix) => openEditor(null, prefix)}
      />
      {editor !== undefined && (
        <IPv6Editor
          key={editor?.config.id ?? defaultPrefix?.id ?? "new-proxy"}
          initial={editor?.config}
          legacy={Boolean(editor?.legacyEndpointID)}
          prefixes={prefixes}
          defaultPrefix={defaultPrefix}
          endpoints={endpoints}
          interfaces={interfacesQuery.data ?? []}
          pending={pending}
          onClose={() => setEditor(undefined)}
          onSave={(items) => save.mutate({ entry: editor, items })}
        />
      )}

      {credentials.length > 0 && (
        <Card className="platform-list-card">
          <div className="list-card-header">
            <h3>{t("本次保存的 SOCKS5 凭据")}</h3>
            <Button size="sm" variant="secondary" onClick={() => setCredentials([])}>{t("关闭")}</Button>
          </div>
          <label className="field-group field-label">
            {t("用户名、密码、IPv6 地址")}
            <Textarea
              readOnly
              rows={Math.min(credentials.length + 1, 8)}
              value={credentials.map(({ username, password, address }) => [username, password, address].join("\t")).join("\n")}
              onFocus={(event) => event.target.select()}
            />
          </label>
        </Card>
      )}

      <Card className="platform-list-card">
        <div className="list-card-header">
          <div>
            <h3>{t("IPv6 代理")}</h3>
            <p>{t("{{count}} 个代理", { count: entries.length })}</p>
          </div>
        </div>
        {loading ? <p className="muted">{t("加载中...")}</p> : entries.length === 0 ? (
          <p className="muted">{t("暂无 IPv6 配置")}</p>
        ) : (
          <div className="data-table-wrap">
            <table className="data-table ipv6-table">
              <thead>
                <tr>
                  <th>{t("SOCKS5 用户名")}</th>
                  <th>{t("IPv6 地址")}</th>
                  <th>{t("号段标签")}</th>
                  <th>{t("连接端口")}</th>
                  <th>{t("操作")}</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => {
                  const { config } = entry;
                  const ports = config.endpoint_ids.flatMap((id) => {
                    const port = portByID.get(id);
                    return port === undefined ? [] : [port];
                  });
                  const prefix = config.prefix_id ? prefixByID.get(config.prefix_id) : undefined;
                  return (
                    <tr key={config.id}>
                      <td>
                        {config.username}
                        {entry.legacyEndpointID && <small className="muted ipv6-legacy">{t("旧版映射")}</small>}
                      </td>
                      <td><code>{config.address}</code></td>
                      <td>
                        <Badge variant={prefix ? "accent" : "muted"}>{prefix?.label ?? t("独立地址")}</Badge>
                        <small className="muted ipv6-legacy">{config.interface}</small>
                      </td>
                      <td>
                        <div className="ipv6-port-badges">
                          {ports.length ? ports.map((port) => <Badge key={port} variant="info">:{port}</Badge>) :
                            <Badge variant="muted">{t("未绑定")}</Badge>}
                        </div>
                      </td>
                      <td>
                        <div className="detail-actions">
                          <Button variant="secondary" size="sm" disabled={proxyFormUnavailable} onClick={() => openEditor(entry)}>
                            <Pencil size={14} />
                            {t("编辑")}
                          </Button>
                          <Button variant="danger" size="sm" disabled={pending || editor?.config.id === config.id} onClick={() => {
                            if (window.confirm(t("删除 IPv6 配置 {{username}}？", { username: config.username }))) {
                              remove.mutate(entry);
                            }
                          }}>
                            <Trash2 size={14} />
                            {t("删除")}
                          </Button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </section>
  );
}
