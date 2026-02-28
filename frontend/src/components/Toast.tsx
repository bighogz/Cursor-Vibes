import { cn } from "../lib/cn";
import { useToast } from "../context/toast-context";
import { IconX } from "./icons";

const typeStyles = {
  info: "border-line bg-surface-2 text-content",
  success: "border-positive/30 bg-positive-dim text-positive",
  error: "border-negative/30 bg-negative-dim text-negative",
} as const;

export function ToastContainer() {
  const { toasts, dismiss } = useToast();
  if (toasts.length === 0) return null;

  return (
    <div
      className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2 max-w-sm"
      aria-live="polite"
    >
      {toasts.map((t) => (
        <div
          key={t.id}
          className={cn(
            "flex items-center gap-3 px-3.5 py-2.5 rounded-lg border text-[13px]",
            "shadow-lg shadow-black/20 animate-in",
            typeStyles[t.type]
          )}
        >
          <span className="flex-1">{t.message}</span>
          <button
            onClick={() => dismiss(t.id)}
            className="text-content-muted hover:text-content transition-colors"
            aria-label="Dismiss"
          >
            <IconX size={14} />
          </button>
        </div>
      ))}
    </div>
  );
}
