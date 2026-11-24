import { X } from "lucide-react";
import { useEffect, useId, useRef, type ReactNode } from "react";
import { Button } from "../../components/ui/Button";
import { useI18n } from "../../i18n";

type Props = {
  title: string;
  onClose: () => void;
  children: ReactNode;
};

export function IPv6Dialog({ title, onClose, children }: Props) {
  const { t } = useI18n();
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    dialog.current?.showModal();
  }, []);

  return (
    <dialog ref={dialog} className="card modal-card ipv6-dialog" aria-labelledby={titleId} onClose={onClose}>
      <div className="modal-header">
        <h3 id={titleId}>{title}</h3>
        <Button variant="ghost" size="sm" aria-label={t("关闭")} onClick={() => dialog.current?.close()}>
          <X size={18} aria-hidden="true" />
        </Button>
      </div>
      {children}
    </dialog>
  );
}
