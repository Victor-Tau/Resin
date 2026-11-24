import { Copy } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Button } from "../../components/ui/Button";
import { Input } from "../../components/ui/Input";
import { useI18n } from "../../i18n";
import type { IPv6Config } from "./api";
import { IPv6Dialog } from "./IPv6Dialog";

type Props = {
  missing: IPv6Config[];
  onClose: () => void;
  onCopy: (passwords: Map<string, string>) => Promise<boolean>;
};

export function IPv6CopyDialog({ missing, onClose, onCopy }: Props) {
  const { t } = useI18n();
  const [copying, setCopying] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    const passwords = new Map(missing.map(({ id }) => [id, String(fields.get(id) ?? "")]));
    setCopying(true);
    setCopyFailed(false);
    if (await onCopy(passwords)) {
      onClose();
    } else {
      setCopying(false);
      setCopyFailed(true);
    }
  }

  return (
    <IPv6Dialog title={t("输入 SOCKS5 密码")} onClose={onClose}>
      <form className="ipv6-copy-form" onSubmit={submit}>
        <p className="muted">{t("服务器只保存密码哈希。请输入现有密码，复制后会在此标签页的会话中保留。")}</p>
        {copyFailed && <p className="callout callout-error" role="alert">{t("无法访问剪贴板，请重试。")}</p>}
        <fieldset className="ipv6-fieldset" disabled={copying}>
          {missing.map(({ id, username }) => (
            <label className="field-group field-label" key={id}>
              {t("{{username}} 的 SOCKS5 密码", { username })}
              <Input name={id} type="password" required autoComplete="off" maxLength={255} />
            </label>
          ))}
          <div className="detail-actions">
            <Button type="submit"><Copy size={14} aria-hidden="true" />{t("复制代理链接")}</Button>
            <Button variant="secondary" onClick={onClose}>{t("取消")}</Button>
          </div>
        </fieldset>
      </form>
    </IPv6Dialog>
  );
}
