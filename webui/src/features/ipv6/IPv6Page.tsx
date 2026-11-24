import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Cable, ChevronRight, Copy, KeyRound, Layers, Pencil, Plus, Trash2 } from "lucide-react";
import { useState } from "react";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Card } from "../../components/ui/Card";
import { Textarea } from "../../components/ui/Textarea";
import { ToastContainer } from "../../components/ui/Toast";
import { useToast } from "../../hooks/useToast";
import { useI18n } from "../../i18n";
import { copyText } from "../../lib/clipboard";
import { formatApiErrorMessage } from "../../lib/error-message";
import { getStoredAuthToken } from "../auth/auth-store";
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
import { IPv6CopyDialog } from "./IPv6CopyDialog";
import { IPv6Dialog } from "./IPv6Dialog";
import { IPv6Prefixes } from "./IPv6Prefixes";
import { loadIPv6Passwords, socks5Links, socks5Ports, storeIPv6Passwords } from "./proxyLinks";

type Entry = {
  config: IPv6Config;
  legacyEndpointID?: string;
};

async function changeLegacyAccounts(endpointID: string, usernames: string[], replacement?: SOCKS5Account) {
  const endpoint = await getEndpoint(endpointID);
  const names = new Set(usernames);
  if (!endpoint.socks5_accounts.some((account) => names.has(account.username))) {
    throw new Error("IPv6 配置已不存在");
  }
  const accounts = endpoint.socks5_accounts.flatMap((account) =>
    names.has(account.username) ? (replacement ? [replacement] : []) : [account],
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
  const [prefixManagerOpen, setPrefixManagerOpen] = useState(false);
  const [selectedIDs, setSelectedIDs] = useState(() => new Set<string>());
  const [passwords, setPasswords] = useState(loadIPv6Passwords);
  const [credentials, setCredentials] = useState<IPv6Config[]>([]);
  const [copyRequest, setCopyRequest] = useState<IPv6Config[]>();
  const [adminToken] = useState(getStoredAuthToken);
  const configsQuery = useQuery({ queryKey: ["ipv6-configs"], queryFn: listIPv6Configs, gcTime: 0 });
  const endpointsQuery = useQuery({ queryKey: ["endpoints", "ipv6"], queryFn: listIPv6Endpoints, gcTime: 0 });
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
          password: account.password,
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

  function rememberPasswords(next: Map<string, string>) {
    setPasswords(next);
    if (adminToken === getStoredAuthToken()) {
      storeIPv6Passwords(next);
    }
  }

  const save = useMutation({
    gcTime: 0,
    mutationFn: async ({ entry, items }: { entry: Entry | null; items: IPv6ConfigInput[] }): Promise<IPv6Config[]> => {
      if (entry?.legacyEndpointID) {
        const { username, password, address } = items[0];
        await changeLegacyAccounts(entry.legacyEndpointID, [entry.config.username], { username, password, address });
        return [{
          ...entry.config,
          id: `legacy:${entry.legacyEndpointID}:${username}`,
          username,
          password: password || entry.config.password,
          address,
        }];
      }
      return entry ? [await updateIPv6Config(entry.config.id, items[0])] : createIPv6Configs(items);
    },
    onSuccess: async (saved, { entry }) => {
      const next = new Map(passwords);
      const previousID = entry?.config.id;
      if (previousID && previousID !== saved[0].id) {
        next.delete(previousID);
        if (selectedIDs.has(previousID)) {
          setSelectedIDs(new Set([...selectedIDs].filter((id) => id !== previousID).concat(saved[0].id)));
        }
      }
      saved.forEach(({ id, password }) => {
        if (password) {
          next.delete(id);
        }
      });
      rememberPasswords(next);
      setCredentials(saved.filter(({ id, password }) => password || next.has(id)));
      setEditor(undefined);
      await invalidate();
      showToast("success", t("IPv6 配置已保存"));
    },
    onError: (error) => showToast("error", formatApiErrorMessage(error, t)),
  });

  const remove = useMutation({
    mutationFn: async (targets: Entry[]) => {
      const groups = new Map<string, Entry[]>();
      const tasks: { ids: string[]; run: () => Promise<void> }[] = [];
      for (const entry of targets) {
        if (entry.legacyEndpointID) {
          const group = groups.get(entry.legacyEndpointID) ?? [];
          group.push(entry);
          groups.set(entry.legacyEndpointID, group);
        } else {
          tasks.push({ ids: [entry.config.id], run: () => deleteIPv6Config(entry.config.id) });
        }
      }
      for (const [endpointID, group] of groups) {
        tasks.push({
          ids: group.map(({ config }) => config.id),
          run: () => changeLegacyAccounts(endpointID, group.map(({ config }) => config.username)),
        });
      }
      const deleted = new Set<string>();
      const errors: unknown[] = [];
      for (const task of tasks) {
        try {
          await task.run();
          task.ids.forEach((id) => deleted.add(id));
        } catch (error) {
          errors.push(error);
        }
      }
      return { deleted, errors };
    },
    onSuccess: async ({ deleted, errors }, targets) => {
      if (deleted.size) {
        const remaining = new Map(passwords);
        deleted.forEach((id) => remaining.delete(id));
        rememberPasswords(remaining);
        setCredentials((saved) => saved.filter(({ id }) => !deleted.has(id)));
        setSelectedIDs((selected) => new Set(Array.from(selected).filter((id) => !deleted.has(id))));
        setEditor((current) => current && deleted.has(current.config.id) ? undefined : current);
      }
      await invalidate();
      if (errors.length) {
        const firstError = formatApiErrorMessage(errors[0], t);
        showToast("error", targets.length === 1 ? firstError : t("已删除 {{deleted}} 个，{{failed}} 个删除失败。{{reason}}", {
          deleted: deleted.size, failed: targets.length - deleted.size, reason: firstError,
        }));
      } else {
        showToast("success", targets.length === 1 ? t("IPv6 配置已删除") : t("已删除 {{count}} 个 IPv6 代理", { count: deleted.size }));
      }
    },
    onError: (error) => showToast("error", formatApiErrorMessage(error, t)),
  });

  const loading = configsQuery.isPending || endpointsQuery.isPending;
  const pending = save.isPending || remove.isPending;
  const error = configsQuery.error ?? endpointsQuery.error ?? prefixesQuery.error ?? interfacesQuery.error;
  const portByID = new Map(endpoints.map((endpoint) => [endpoint.id, endpoint.port]));
  const prefixByID = new Map(prefixes.map((prefix) => [prefix.id, prefix]));
  const proxyFormUnavailable = pending || endpointsQuery.isPending || prefixesQuery.isPending;
  const selectedEntries = entries.filter(({ config }) => selectedIDs.has(config.id));
  const allSelected = entries.length > 0 && selectedEntries.length === entries.length;
  const missingPasswords = copyRequest?.filter(
    (config) => socks5Ports(config, endpoints).length && !config.password && !passwords.has(config.id),
  ) ?? [];
  const savedLinks = credentials.flatMap((config) => {
    const password = config.password || passwords.get(config.id);
    return password ? socks5Links(config, password, socks5Ports(config, endpoints)) : [];
  }).join("\n");
  const savedWithoutPorts = credentials.filter((config) => !socks5Ports(config, endpoints).length).length;

  function openEditor(entry: Entry | null, prefix?: IPv6Prefix) {
    setDefaultPrefix(prefix);
    setEditor(entry);
    requestAnimationFrame(() => document.getElementById("ipv6-proxy-editor")?.scrollIntoView({ block: "nearest" }));
  }

  async function copy(value: string, feedback = t("已复制")) {
    try {
      await copyText(value);
      showToast("success", feedback);
      return true;
    } catch {
      showToast("error", t("无法访问剪贴板，请重试。"));
      return false;
    }
  }

  async function copyProxies(configs: IPv6Config[], supplied?: Map<string, string>) {
    const bound = configs.map((config) => ({ config, ports: socks5Ports(config, endpoints) }))
      .filter(({ ports }) => ports.length);
    if (!bound.length) {
      showToast("error", t("请先绑定已启用的 SOCKS5 端口。"));
      return false;
    }
    const known = new Map(passwords);
    supplied?.forEach((password, id) => known.set(id, password));
    if (bound.some(({ config }) => !config.password && !known.has(config.id))) {
      setCopyRequest(configs);
      return false;
    }
    const links = bound.flatMap(({ config, ports }) => socks5Links(config, config.password || known.get(config.id) || "", ports));
    const skipped = configs.length - bound.length;
    const copied = await copy(links.join("\n"), skipped
      ? t("已复制 {{count}} 条链接，{{skipped}} 个代理没有可用的 SOCKS5 端口。", { count: links.length, skipped })
      : t("已复制 {{count}} 条 SOCKS5 链接", { count: links.length }));
    if (copied && supplied) {
      rememberPasswords(known);
    }
    return copied;
  }

  function toggleSelected(id: string, checked: boolean) {
    setSelectedIDs((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  }

  function confirmDelete(targets: Entry[]) {
    const message = targets.length === 1
      ? t("删除 IPv6 配置 {{username}}？", { username: targets[0].config.username })
      : t("删除选中的 {{count}} 个 IPv6 代理？", { count: targets.length });
    if (window.confirm(message)) {
      remove.mutate(targets);
    }
  }

  const flow = [
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
        <li className="ipv6-flow-trigger">
          <button type="button" className="ipv6-flow-button" aria-haspopup="dialog" onClick={() => setPrefixManagerOpen(true)}>
            <span className="ipv6-icon-tile"><Layers size={19} aria-hidden="true" /></span>
            <span>
              <strong>{t("号段管理")}</strong>
              <small className="muted">{prefixesQuery.isPending ? t("加载中...") : t("{{count}} 个已保存号段", { count: prefixes.length })}</small>
            </span>
            <ChevronRight className="ipv6-flow-arrow" size={17} aria-hidden="true" />
          </button>
        </li>
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
      {prefixManagerOpen && (
        <IPv6Dialog title={t("号段管理")} onClose={() => setPrefixManagerOpen(false)}>
          <IPv6Prefixes
            prefixes={prefixes}
            interfaces={interfacesQuery.data ?? []}
            loading={prefixesQuery.isPending}
            detecting={interfacesQuery.isFetching}
            onDetect={() => void interfacesQuery.refetch()}
            proxyPending={pending}
            canCreateProxy={!proxyFormUnavailable}
            onUse={(prefix) => {
              setPrefixManagerOpen(false);
              openEditor(null, prefix);
            }}
          />
        </IPv6Dialog>
      )}
      {copyRequest && missingPasswords.length > 0 && (
        <IPv6CopyDialog
          missing={missingPasswords}
          onClose={() => setCopyRequest(undefined)}
          onCopy={(supplied) => copyProxies(copyRequest, supplied)}
        />
      )}
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
          <div className="list-card-header ipv6-section-header">
            <h3>{t("本次保存的 SOCKS5 凭据")}</h3>
            <div className="detail-actions">
              <Button size="sm" variant="secondary" disabled={!savedLinks} onClick={() => void copy(savedLinks)}>
                <Copy size={14} aria-hidden="true" />{t("复制全部链接")}
              </Button>
              <Button size="sm" variant="ghost" onClick={() => setCredentials([])}>{t("关闭")}</Button>
            </div>
          </div>
          {savedLinks && (
            <Textarea
              aria-label={t("本次保存的 SOCKS5 凭据")}
              className="ipv6-credential-links"
              readOnly
              rows={Math.min(savedLinks.split("\n").length + 1, 8)}
              value={savedLinks}
              onFocus={(event) => event.target.select()}
            />
          )}
          {savedWithoutPorts > 0 && <p className="muted">{t("{{count}} 个代理尚无可用的 SOCKS5 端口。", { count: savedWithoutPorts })}</p>}
        </Card>
      )}

      <Card className="platform-list-card">
        <div className="list-card-header ipv6-section-header">
          <div>
            <h3>{t("IPv6 代理")}</h3>
            <p>{t("{{count}} 个代理", { count: entries.length })}</p>
          </div>
          {entries.length > 0 && (
            <div className="detail-actions">
              {selectedEntries.length > 0 && <span className="muted">{t("已选 {{count}} 个", { count: selectedEntries.length })}</span>}
              <Button size="sm" variant="secondary" disabled={pending || !selectedEntries.length}
                onClick={() => void copyProxies(selectedEntries.map(({ config }) => config))}>
                <Copy size={14} aria-hidden="true" />{t("批量复制")}
              </Button>
              <Button size="sm" variant="danger" disabled={pending || !selectedEntries.length} onClick={() => confirmDelete(selectedEntries)}>
                <Trash2 size={14} aria-hidden="true" />{t("批量删除")}
              </Button>
            </div>
          )}
        </div>
        {loading ? <p className="muted">{t("加载中...")}</p> : entries.length === 0 ? (
          <p className="muted">{t("暂无 IPv6 配置")}</p>
        ) : (
          <div className="data-table-wrap">
            <table className="data-table ipv6-table">
              <thead>
                <tr>
                  <th className="ipv6-select-cell">
                    <input type="checkbox" aria-label={t("选择全部 IPv6 代理")} checked={allSelected} disabled={pending}
                      ref={(input) => { if (input) input.indeterminate = selectedEntries.length > 0 && !allSelected; }}
                      onChange={(event) => setSelectedIDs(new Set(event.target.checked ? entries.map(({ config }) => config.id) : []))} />
                  </th>
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
                  const address = config.address.split("/", 1)[0];
                  const ports = config.endpoint_ids.flatMap((id) => {
                    const port = portByID.get(id);
                    return port === undefined ? [] : [port];
                  });
                  const prefix = config.prefix_id ? prefixByID.get(config.prefix_id) : undefined;
                  return (
                    <tr key={config.id}>
                      <td className="ipv6-select-cell">
                        <input type="checkbox" aria-label={t("选择代理 {{username}}", { username: config.username })}
                          checked={selectedIDs.has(config.id)} disabled={pending} onChange={(event) => toggleSelected(config.id, event.target.checked)} />
                      </td>
                      <td>
                        {config.username}
                        {entry.legacyEndpointID && <small className="muted ipv6-legacy">{t("旧版映射")}</small>}
                      </td>
                      <td>
                        <div className="ipv6-address-cell">
                          <code>{address}</code>
                          <Button variant="ghost" size="sm" aria-label={t("复制 IPv6 地址")} title={t("复制 IPv6 地址")}
                            onClick={() => void copy(address)}>
                            <Copy size={14} aria-hidden="true" />
                          </Button>
                        </div>
                      </td>
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
                          <Button variant="secondary" size="sm" disabled={pending} onClick={() => void copyProxies([config])}>
                            <Copy size={14} aria-hidden="true" />
                            {t("复制")}
                          </Button>
                          <Button variant="secondary" size="sm" disabled={proxyFormUnavailable} onClick={() => openEditor(entry)}>
                            <Pencil size={14} />
                            {t("编辑")}
                          </Button>
                          <Button variant="danger" size="sm" disabled={pending} onClick={() => confirmDelete([entry])}>
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
