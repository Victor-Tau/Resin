import { useState, type FormEvent } from "react";
import { Cable, KeyRound, Layers } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { Card } from "../../components/ui/Card";
import { Input } from "../../components/ui/Input";
import { useI18n } from "../../i18n";
import type { Endpoint, SOCKS5Account } from "../endpoints/types";
import type { IPv6Config, IPv6ConfigInput, IPv6Interface, IPv6Prefix } from "./api";
import { generateAddress, generatePassword, ipv6Network } from "./generate";

const manualPrefix = "manual";
const batchSizes = Array.from({ length: 20 }, (_, index) => index + 1);

type Props = {
  initial?: IPv6Config;
  legacy: boolean;
  prefixes: IPv6Prefix[];
  defaultPrefix?: IPv6Prefix;
  endpoints: Endpoint[];
  interfaces: IPv6Interface[];
  pending: boolean;
  onClose: () => void;
  onSave: (items: IPv6ConfigInput[]) => void;
};

export function IPv6Editor({ initial, legacy, prefixes, defaultPrefix, endpoints, interfaces, pending, onClose, onSave }: Props) {
  const { t } = useI18n();
  const [prefixID, setPrefixID] = useState(initial?.prefix_id ?? defaultPrefix?.id ?? (initial ? manualPrefix : ""));
  const [iface, setIface] = useState(initial?.interface ?? "");
  const [selectedPrefix, setSelectedPrefix] = useState("");
  const [customPrefix, setCustomPrefix] = useState("");
  const [bindings, setBindings] = useState(initial?.endpoint_ids ?? []);
  const [accounts, setAccounts] = useState<SOCKS5Account[]>(() => {
    if (initial) {
      return [{ username: initial.username, address: initial.address }];
    }
    const network = defaultPrefix && ipv6Network(defaultPrefix.prefix);
    return [{ username: "", address: network ? generateAddress(network, new Set()) : "", password: generatePassword() }];
  });
  const [prefixError, setPrefixError] = useState(false);

  const saved = prefixes.find(({ id }) => id === prefixID);
  const manual = legacy || prefixID === manualPrefix;
  const detected = interfaces.find((item) => item.name === iface)?.prefixes.filter((prefix) => ipv6Network(prefix)) ?? [];
  const bindable = endpoints.filter((endpoint) => endpoint.allow_socks5 || bindings.includes(endpoint.id));
  const network = ipv6Network(saved?.prefix ?? (selectedPrefix === manualPrefix ? customPrefix : selectedPrefix));

  function selectSavedPrefix(id: string) {
    setPrefixID(id);
    setPrefixError(false);
    const next = prefixes.find((item) => item.id === id);
    if (next) {
      regenerateAddresses(next.prefix);
    } else if (id === manualPrefix && saved) {
      setIface(saved.interface);
      setSelectedPrefix(manualPrefix);
      setCustomPrefix(saved.prefix);
    }
  }

  function regenerateAddresses(prefix: string) {
    const nextNetwork = ipv6Network(prefix);
    setPrefixError(nextNetwork === null);
    if (nextNetwork) {
      const used = new Set(accounts.map((account) => account.address));
      setAccounts(accounts.map((account) => ({ ...account, address: generateAddress(nextNetwork, used) })));
    }
  }

  function changeBatchSize(count: number) {
    const next = accounts.slice(0, count);
    const used = new Set(next.map((account) => account.address));
    while (next.length < count) {
      next.push({
        username: "",
        password: generatePassword(),
        address: network ? generateAddress(network, used) : "",
      });
    }
    setAccounts(next);
  }

  function setAccount(index: number, changes: Partial<SOCKS5Account>) {
    setAccounts(accounts.map((account, position) => position === index ? { ...account, ...changes } : account));
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!saved && !manual) {
      return;
    }
    onSave(accounts.map((account) => ({
      ...account,
      username: legacy ? account.username : account.username.trim(),
      address: account.address.trim(),
      interface: saved?.interface ?? iface,
      prefix_id: saved?.id,
      endpoint_ids: bindings,
    })));
  }

  return (
    <Card className="platform-list-card ipv6-section" id="ipv6-proxy-editor">
      <div className="list-card-header">
        <h3>{initial ? t("编辑代理") : t("创建 IPv6 代理")}</h3>
      </div>
      <form onSubmit={submit}>
        <fieldset className="form-grid ipv6-fieldset" disabled={pending}>
          <h4 className="ipv6-form-heading field-span-2"><Layers size={17} aria-hidden="true" />{t("选择出口号段")}</h4>
          {!legacy && (
            <div className="field-group field-span-2">
              <label className="field-label" htmlFor="ipv6-saved-prefix">{t("已保存的号段")}</label>
              <select id="ipv6-saved-prefix" className="input" value={prefixID} required
                onChange={(event) => selectSavedPrefix(event.target.value)}>
                <option value="">{t("按标签选择号段")}</option>
                {prefixID && !saved && !manual && <option value={prefixID} disabled>{t("所选号段已删除，请重新选择")}</option>}
                {prefixes.map(({ id, label, prefix, interface: name }) => (
                  <option key={id} value={id}>{label} · {prefix} · {name}</option>
                ))}
                <option value={manualPrefix}>{t("高级：手动配置出口")}</option>
              </select>
              <small className="muted">{prefixes.length
                ? t("选定号段后，会为每个代理随机分配一个 IPv6 地址。")
                : t("可先在上方添加号段，再回到这里选择。")}</small>
              {saved && (
                <Button variant="secondary" size="sm" className="ipv6-regenerate" onClick={() => regenerateAddresses(saved.prefix)}>
                  {t("随机生成 IPv6 地址")}
                </Button>
              )}
            </div>
          )}
          {manual && (
            <>
              <label className="field-group field-label">
                {t("SOCKS5 出口网卡")}
                <select
                  className="input"
                  value={iface}
                  required
                  disabled={legacy}
                  onChange={(event) => {
                    setIface(event.target.value);
                    setSelectedPrefix("");
                    setPrefixError(false);
                  }}
                >
                  <option value="">{t("选择网卡")}</option>
                  {iface && !interfaces.some((item) => item.name === iface) && <option value={iface}>{iface}</option>}
                  {interfaces.map(({ name }) => <option key={name} value={name}>{name}</option>)}
                </select>
              </label>
              <label className="field-group field-label">
                {t("IPv6 /64 号段")}
                <select
                  className="input"
                  value={selectedPrefix}
                  disabled={!iface}
                  onChange={(event) => {
                    const prefix = event.target.value;
                    setSelectedPrefix(prefix);
                    setPrefixError(false);
                    if (prefix && prefix !== manualPrefix) {
                      regenerateAddresses(prefix);
                    }
                  }}
                >
                  <option value="">{t("选择 /64 号段")}</option>
                  {detected.map((prefix) => <option key={prefix} value={prefix}>{prefix}</option>)}
                  <option value={manualPrefix}>{t("手动输入 /64 号段")}</option>
                </select>
              </label>
              {selectedPrefix === manualPrefix && (
                <label className="field-group field-label">
                  {t("手动输入 /64 号段")}
                  <Input
                    value={customPrefix}
                    placeholder="2001:db8:1234:5601::/64"
                    onChange={(event) => {
                      setCustomPrefix(event.target.value);
                      setPrefixError(false);
                    }}
                  />
                </label>
              )}
              {selectedPrefix && (
                <div className="ipv6-generate-address">
                  <Button variant="secondary" onClick={() => regenerateAddresses(
                    selectedPrefix === manualPrefix ? customPrefix : selectedPrefix,
                  )}>
                    {t("随机生成 IPv6 地址")}
                  </Button>
                </div>
              )}
              {prefixError && (
                <p className="callout callout-error field-span-2" role="alert">{t("请输入有效的公网 IPv6 /64 号段")}</p>
              )}
            </>
          )}
          <h4 className="ipv6-form-heading field-span-2"><KeyRound size={17} aria-hidden="true" />{t("设置登录账号")}</h4>
          {!initial && (
            <label className="field-group field-label ipv6-batch-size">
              {t("添加数量")}
              <select className="input" value={accounts.length} onChange={(event) => changeBatchSize(Number(event.target.value))}>
                {batchSizes.map((count) => <option key={count} value={count}>{count}</option>)}
              </select>
            </label>
          )}
          <div className="field-span-2">
            {accounts.map((account, index) => {
              const retainsPassword = initial && (!legacy || initial.username === account.username);
              return (
                <div className="ipv6-account-fields" key={index}>
                  <label className="field-group field-label">
                    {t("SOCKS5 用户名")}
                    <Input
                      required
                      maxLength={255}
                      placeholder={t("例如：proxy01")}
                      value={account.username}
                      onChange={(event) => setAccount(index, { username: event.target.value })}
                    />
                  </label>
                  <div className="field-group">
                    <label className="field-label" htmlFor={`ipv6-password-${index}`}>{t("SOCKS5 密码")}</label>
                    <div className="ipv6-password">
                      <Input
                        id={`ipv6-password-${index}`}
                        autoComplete="off"
                        maxLength={255}
                        required={!retainsPassword}
                        placeholder={retainsPassword ? t("留空保留原密码") : undefined}
                        value={account.password ?? ""}
                        onChange={(event) => setAccount(index, { password: event.target.value })}
                      />
                      <Button size="sm" variant="secondary" onClick={() => setAccount(index, { password: generatePassword() })}>
                        {t("随机生成")}
                      </Button>
                    </div>
                  </div>
                  <label className="field-group field-label">
                    {t("固定出口 IPv6 地址（含 /64）")}
                    <Input
                      required
                      placeholder="2001:db8:1234:5601::10/64"
                      value={account.address}
                      onChange={(event) => setAccount(index, { address: event.target.value })}
                    />
                  </label>
                </div>
              );
            })}
          </div>
          {!legacy && (
            <fieldset className="ipv6-fieldset field-span-2">
              <legend className="ipv6-form-heading"><Cable size={17} aria-hidden="true" />{t("选择连接端口")}</legend>
              <p className="muted">{t("接入点是代理客户端连接的端口。可以勾选多个，也可以稍后再选。")}</p>
              <div className="ipv6-bindings">
                {bindable.map((endpoint) => (
                  <label key={endpoint.id}>
                    <input
                      type="checkbox"
                      checked={bindings.includes(endpoint.id)}
                      onChange={(event) => setBindings(
                        event.target.checked
                          ? [...bindings, endpoint.id]
                          : bindings.filter((id) => id !== endpoint.id),
                      )}
                    />
                    :{endpoint.port}{endpoint.read_only ? ` · ${t("默认")}` : ""}
                    {!endpoint.enabled || !endpoint.allow_socks5 ? ` · ${t("未生效")}` : ""}
                  </label>
                ))}
              </div>
              {bindings.length > 0 && <p className="muted">{t("绑定后，这些端口的 SOCKS5 客户端需要使用 IPv6 代理的账号和密码。")}</p>}
            </fieldset>
          )}
          <div className="detail-actions field-span-2">
            <Button variant="secondary" onClick={onClose}>{t("取消")}</Button>
            <Button type="submit" disabled={!saved && !manual}>{pending ? t("保存中...") : t("保存配置")}</Button>
          </div>
        </fieldset>
      </form>
    </Card>
  );
}
