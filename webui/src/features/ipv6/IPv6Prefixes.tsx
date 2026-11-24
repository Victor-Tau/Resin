import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ArrowRight, Layers, Pencil, Plus, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Badge } from "../../components/ui/Badge";
import { Button } from "../../components/ui/Button";
import { Card } from "../../components/ui/Card";
import { Input } from "../../components/ui/Input";
import { useI18n } from "../../i18n";
import { formatApiErrorMessage } from "../../lib/error-message";
import {
  deleteIPv6Prefix,
  saveIPv6Prefix,
  type IPv6Interface,
  type IPv6Prefix,
  type IPv6PrefixInput,
} from "./api";
import { ipv6Network } from "./generate";

type Props = {
  prefixes: IPv6Prefix[];
  interfaces: IPv6Interface[];
  loading: boolean;
  proxyPending: boolean;
  canCreateProxy: boolean;
  onUse: (prefix: IPv6Prefix) => void;
};

export function IPv6Prefixes({ prefixes, interfaces, loading, proxyPending, canCreateProxy, onUse }: Props) {
  const { t } = useI18n();
  const client = useQueryClient();
  const [editing, setEditing] = useState<IPv6Prefix | null>();
  const [problem, setProblem] = useState<unknown>(null);

  const refresh = () => Promise.all([
    client.invalidateQueries({ queryKey: ["ipv6-prefixes"] }),
    client.invalidateQueries({ queryKey: ["ipv6-configs"] }),
  ]);

  const save = useMutation({
    mutationFn: (input: IPv6PrefixInput) => saveIPv6Prefix(input, editing?.id),
    onSuccess: async () => {
      await refresh();
      setEditing(undefined);
    },
    onError: setProblem,
  });
  const remove = useMutation({
    mutationFn: deleteIPv6Prefix,
    onSuccess: refresh,
    onError: setProblem,
  });

  const busy = proxyPending || save.isPending || remove.isPending;

  function open(prefix: IPv6Prefix | null) {
    setProblem(null);
    setEditing(prefix);
  }

  return (
    <Card className="platform-list-card ipv6-section" id="ipv6-prefixes">
      <div className="list-card-header ipv6-section-header">
        <div>
          <h3>{t("号段管理")}</h3>
          <p>{t("号段是一组可分配给代理的 IPv6 地址。给它起个好记的名字，创建代理时就能直接选用。")}</p>
        </div>
        <Button onClick={() => open(null)} disabled={busy || editing === null}>
          <Plus size={16} />
          {t("添加号段")}
        </Button>
      </div>

      {problem != null && <div className="callout callout-error" role="alert">{formatApiErrorMessage(problem, t)}</div>}
      {editing !== undefined && (
        <PrefixEditor
          key={editing?.id ?? "new-prefix"}
          initial={editing}
          interfaces={interfaces}
          busy={busy}
          onClose={() => { setEditing(undefined); setProblem(null); }}
          onSave={(input) => { setProblem(null); save.mutate(input); }}
        />
      )}

      {loading && <p className="muted">{t("加载中...")}</p>}
      {!loading && prefixes.length === 0 && editing === undefined && (
        <div className="ipv6-empty">
          <Layers size={24} aria-hidden="true" />
          <div>
            <strong>{t("还没有保存的号段")}</strong>
            <p className="muted">{t("点击「添加号段」，可以从服务器检测到的号段中选择。")}</p>
          </div>
        </div>
      )}

      {prefixes.length > 0 && (
        <div className="ipv6-prefix-grid">
          {prefixes.map((prefix) => (
            <article key={prefix.id} className="ipv6-prefix-card">
              <div className="ipv6-prefix-card-heading">
                <span className="ipv6-icon-tile"><Layers size={16} aria-hidden="true" /></span>
                <h4>{prefix.label}</h4>
                <Badge variant={prefix.proxy_count ? "info" : "muted"}>
                  {t("{{count}} 个代理", { count: prefix.proxy_count })}
                </Badge>
              </div>
              <code className="ipv6-prefix-address">{prefix.prefix}</code>
              <p className="muted">{t("出口网卡 {{name}}", { name: prefix.interface })}</p>
              <div className="ipv6-card-actions">
                <Button size="sm" variant="secondary" onClick={() => onUse(prefix)} disabled={busy || !canCreateProxy || editing !== undefined}>
                  {t("用此号段创建代理")}
                  <ArrowRight size={14} aria-hidden="true" />
                </Button>
                <Button size="sm" variant="ghost" onClick={() => open(prefix)} disabled={busy}
                  aria-label={t("编辑号段 {{label}}", { label: prefix.label })}>
                  <Pencil size={14} aria-hidden="true" />
                  {t("编辑")}
                </Button>
                <Button size="sm" variant="ghost" onClick={() => {
                  if (window.confirm(t("删除号段「{{label}}」？已有代理仍可使用原地址。", { label: prefix.label }))) {
                    setProblem(null);
                    remove.mutate(prefix.id);
                  }
                }} disabled={busy || editing?.id === prefix.id}
                  aria-label={t("删除号段 {{label}}", { label: prefix.label })}>
                  <Trash2 size={14} aria-hidden="true" />
                  {t("删除")}
                </Button>
              </div>
            </article>
          ))}
        </div>
      )}
    </Card>
  );
}

type EditorProps = {
  initial: IPv6Prefix | null;
  interfaces: IPv6Interface[];
  busy: boolean;
  onClose: () => void;
  onSave: (input: IPv6PrefixInput) => void;
};

function PrefixEditor({ initial, interfaces, busy, onClose, onSave }: EditorProps) {
  const { t } = useI18n();
  const detected = interfaces.flatMap(({ name, prefixes }) =>
    prefixes.filter((prefix) => ipv6Network(prefix)).map((prefix) => ({ interface: name, prefix, key: `${name}/${prefix}` })),
  );
  const current = detected.find((item) => item.interface === initial?.interface && item.prefix === initial?.prefix);
  const [choice, setChoice] = useState(current?.key ?? (initial ? "manual" : ""));
  const [label, setLabel] = useState(initial?.label ?? "");
  const [iface, setIface] = useState(initial?.interface ?? "");
  const [prefix, setPrefix] = useState(initial?.prefix ?? "");
  const [invalid, setInvalid] = useState(false);
  const fixed = Boolean(initial?.proxy_count);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const selection = fixed ? initial : choice === "manual" ? { interface: iface, prefix } : detected.find(({ key }) => key === choice);
    if (!selection || !selection.interface.trim() || !ipv6Network(selection.prefix) || !label.trim()) {
      setInvalid(true);
      return;
    }
    onSave({ label: label.trim(), interface: selection.interface.trim(), prefix: selection.prefix.trim() });
  }

  return (
    <form className="ipv6-prefix-editor" onSubmit={submit}>
      <fieldset className="form-grid ipv6-fieldset" disabled={busy}>
        <legend>{initial ? t("编辑号段") : t("添加号段")}</legend>
        <label className="field-group field-label">
          {t("标签（自定义名称）")}
          <Input value={label} onChange={(event) => setLabel(event.target.value)} required maxLength={60}
            placeholder={t("例如：家用网络")} />
        </label>
        {fixed ? (
          <div className="field-group">
            <span className="field-label">{t("IPv6 /64 号段")}</span>
            <code>{initial?.prefix}</code>
            <small className="muted">{t("已有代理使用此号段。可以修改标签。")}</small>
          </div>
        ) : (
          <label className="field-group field-label">
            {t("IPv6 /64 号段")}
            <select className="input" value={choice} required onChange={(event) => {
              setChoice(event.target.value);
              setInvalid(false);
            }}>
              <option value="">{t("选择服务器检测到的号段")}</option>
              {detected.map((item) => (
                <option key={item.key} value={item.key}>{item.prefix} · {item.interface}</option>
              ))}
              <option value="manual">{t("手动填写号段")}</option>
            </select>
            {!detected.length && <small className="muted">{t("未检测到 /64 号段。可以手动填写网络服务商分配的号段。")}</small>}
          </label>
        )}
        {!fixed && choice === "manual" && (
          <>
            <label className="field-group field-label">
              {t("出口网卡")}
              <Input value={iface} required onChange={(event) => setIface(event.target.value)}
                list="ipv6-interface-names" placeholder="eth0" />
              <datalist id="ipv6-interface-names">
                {interfaces.map(({ name }) => <option key={name} value={name} />)}
              </datalist>
            </label>
            <label className="field-group field-label">
              {t("手动输入 /64 号段")}
              <Input value={prefix} required onChange={(event) => setPrefix(event.target.value)}
                placeholder="2001:db8:1234:5601::/64" />
            </label>
          </>
        )}
        {invalid && <p className="callout callout-error field-span-2" role="alert">{t("请填写标签、出口网卡和有效的公网 IPv6 /64 号段。")}</p>}
        <div className="detail-actions field-span-2">
          <Button type="submit">{busy ? t("保存中...") : t("保存号段")}</Button>
          <Button variant="secondary" onClick={onClose}>{t("取消")}</Button>
        </div>
      </fieldset>
    </form>
  );
}
