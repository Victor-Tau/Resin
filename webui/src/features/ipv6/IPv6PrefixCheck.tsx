import { useMutation } from "@tanstack/react-query";
import { ScanLine } from "lucide-react";
import { Button } from "../../components/ui/Button";
import { useI18n } from "../../i18n";
import { formatApiErrorMessage } from "../../lib/error-message";
import { checkIPv6Prefix } from "./api";

export function IPv6PrefixCheck({ iface, prefix, disabled }: { iface: string; prefix: string; disabled: boolean }) {
  const { t } = useI18n();
  const check = useMutation({ mutationFn: checkIPv6Prefix });
  const result = (check.variables?.interface === iface && check.variables?.prefix === prefix) ? check.data : undefined;
  const details = result?.error;
  const failed = result?.stage === "cleanup" || !result?.assigned;

  return (
    <div className="field-group">
      <Button size="sm" variant="secondary" className="ipv6-regenerate" disabled={disabled || check.isPending}
        onClick={() => check.mutate({ interface: iface, prefix })}>
        <ScanLine size={15} aria-hidden="true" />
        {check.isPending ? t("正在验证号段…") : t("验证号段可用性")}
      </Button>
      <div aria-live="polite">
        {check.isError && check.variables?.interface === iface && check.variables?.prefix === prefix &&
          <p className="callout callout-error">{formatApiErrorMessage(check.error, t)}</p>}
        {result && (
          <div className={`callout ${failed ? "callout-error" : result.egress ? "callout-success" : "callout-warning"}`}>
            <strong>{result.stage === "cleanup" ? t("临时测试地址清理失败") : !result.assigned
              ? t("无法在网卡添加新地址")
              : result.egress ? t("可分配地址，公网 IPv6 已验证") : t("可添加地址，公网 IPv6 未验证")}</strong>
            {result.address && <div>{t("测试地址")} <code>{result.address}</code></div>}
            {details && <details><summary>{t("查看检测详情")}</summary><p>{details}</p></details>}
          </div>
        )}
      </div>
    </div>
  );
}
