export async function copyText(value: string) {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value);
      return;
    } catch {
      // Fall back when the browser denies clipboard access.
    }
  }

  const focused = document.activeElement;
  const field = document.createElement("textarea");
  field.value = value;
  field.readOnly = true;
  field.style.position = "fixed";
  field.style.opacity = "0";
  (focused?.closest("dialog[open]") ?? document.body).appendChild(field);
  try {
    field.select();
    if (!document.execCommand("copy")) {
      throw new Error("Clipboard unavailable");
    }
  } finally {
    field.remove();
    if (focused instanceof HTMLElement) {
      focused.focus();
    }
  }
}
