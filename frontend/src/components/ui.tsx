import { type ReactNode, useState, useEffect } from "react";
import { AlertTriangle, Check, Copy, RefreshCw, Search, X } from "lucide-react";

type Tone = "neutral" | "success" | "warning" | "danger" | "info";

export function PageHeader({
  eyebrow,
  title,
  description,
  actions
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  actions?: ReactNode;
}) {
  return (
    <header className="page-header">
      <div>
        {eyebrow && <span className="eyebrow">{eyebrow}</span>}
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {actions && <div className="page-actions">{actions}</div>}
    </header>
  );
}

export function Panel({
  title,
  subtitle,
  actions,
  children,
  className = ""
}: {
  title?: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`panel ${className}`}>
      {(title || actions) && (
        <div className="panel-header">
          <div>
            {title && <h2>{title}</h2>}
            {subtitle && <p>{subtitle}</p>}
          </div>
          {actions}
        </div>
      )}
      {children}
    </section>
  );
}

export function StatCard({
  label,
  value,
  detail,
  tone = "neutral",
  extra
}: {
  label: string;
  value: string | number;
  detail?: string;
  tone?: Tone;
  extra?: ReactNode;
}) {
  return (
    <div className={`stat-card tone-${tone}`}>
      <span>
        {label}
        {extra}
      </span>
      <strong>{value}</strong>
      {detail && <small>{detail}</small>}
    </div>
  );
}

export function StatusChip({ value }: { value?: string }) {
  const normalized = value || "unknown";
  let tone: Tone = "neutral";
  if (["online", "acked", "enabled", "healthy"].includes(normalized.toLowerCase())) tone = "success";
  if (["warning", "pending", "available", "downloading", "flashing", "rebooting"].includes(normalized.toLowerCase())) tone = "warning";
  if (["critical", "nacked", "disabled", "error", "timeout", "failed"].includes(normalized.toLowerCase())) tone = "danger";
  if (["ota", "info"].includes(normalized.toLowerCase())) tone = "info";
  return <span className={`status-chip tone-${tone}`}>{normalized}</span>;
}

export function CopyableID({ id, length = 8 }: { id: string; length?: number }) {
  const [copied, setCopied] = useState(false);
  const shortID = id.length > length ? id.slice(0, length) : id;

  async function copyID() {
    try {
      await navigator.clipboard.writeText(id);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      setCopied(false);
    }
  }

  return (
    <span className="copy-id-wrap">
      <button className="copy-id" type="button" onClick={copyID} aria-label={`Copy device ID ${id}`}>
        {shortID}
      </button>
      <span
        className="copy-id-tooltip"
        role="tooltip"
        style={copied ? { display: "inline-flex" } : undefined}
      >
        {copied ? (
          <span style={{ display: "inline-flex", alignItems: "center", gap: "4px" }}>
            <Check size={12} aria-hidden /> Copied
          </span>
        ) : (
          <span style={{ display: "inline-flex", alignItems: "center", gap: "4px" }}>
            <Copy size={12} aria-hidden /> Copy
          </span>
        )}
      </span>
    </span>
  );
}

export function EmptyState({
  title,
  description,
  action
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty-state">
      <AlertTriangle size={20} strokeWidth={1.5} aria-hidden />
      <h3>{title}</h3>
      <p>{description}</p>
      {action}
    </div>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="empty-state error">
      <AlertTriangle size={20} strokeWidth={1.5} aria-hidden />
      <h3>Unable to load data</h3>
      <p>{message}</p>
      {onRetry && (
        <button className="btn-inverse" type="button" onClick={onRetry}>
          <RefreshCw size={15} strokeWidth={1.5} aria-hidden />
          Retry
        </button>
      )}
    </div>
  );
}

export function LoadingRows({ rows = 5 }: { rows?: number }) {
  return (
    <>
      {Array.from({ length: rows }).map((_, index) => (
        <tr className="skeleton-row" key={index}>
          <td colSpan={8}>
            <span />
          </td>
        </tr>
      ))}
    </>
  );
}

export function SelectField({
  label,
  value,
  onChange,
  children
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  children: ReactNode;
}) {
  return (
    <label className="field">
      <span>{label}</span>
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {children}
      </select>
    </label>
  );
}

/* ── New Components ── */

export function FilterTabs({
  options,
  active,
  onChange
}: {
  options: string[];
  active: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="filter-tabs">
      {options.map((option) => (
        <button
          key={option}
          type="button"
          className={active === option ? "active" : ""}
          onClick={() => onChange(option)}
        >
          {option}
        </button>
      ))}
    </div>
  );
}

export function SignalStrength({ strength = 0 }: { strength?: number }) {
  const level = Math.max(0, Math.min(4, strength));
  return (
    <span className={`signal-bars strength-${level}`}>
      <span /><span /><span /><span />
    </span>
  );
}

export function Pagination({
  current,
  total,
  onChange
}: {
  current: number;
  total: number;
  onChange: (page: number) => void;
}) {
  if (total <= 1) return null;
  const pages: (number | "...")[] = [];
  for (let i = 1; i <= total; i++) {
    if (i === 1 || i === total || (i >= current - 1 && i <= current + 1)) {
      pages.push(i);
    } else if (pages[pages.length - 1] !== "...") {
      pages.push("...");
    }
  }
  return (
    <div className="pagination">
      <button type="button" disabled={current === 1} onClick={() => onChange(current - 1)}>← Prev</button>
      {pages.map((page, idx) =>
        page === "..." ? (
          <span className="page-ellipsis" key={`e${idx}`}>…</span>
        ) : (
          <button
            key={page}
            type="button"
            className={page === current ? "active" : ""}
            onClick={() => onChange(page)}
          >
            {page}
          </button>
        )
      )}
      <button type="button" disabled={current === total} onClick={() => onChange(current + 1)}>Next →</button>
    </div>
  );
}

export function LiveIndicator() {
  const [time, setTime] = useState(new Date());
  useEffect(() => {
    const interval = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(interval);
  }, []);
  const timeStr = time.toLocaleTimeString("en-US", { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" });
  const dateStr = time.toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
  return (
    <div className="timestamp-display">
      <span>{timeStr} · {dateStr}</span>
    </div>
  );
}

export function ProgressBar({
  value,
  max
}: {
  value: number;
  max: number;
  color?: string;
}) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0;
  return (
    <div className="progress-track">
      <div className="progress-fill" style={{ width: `${pct}%` }} />
    </div>
  );
}

export function EventStreamEntry({
  time,
  type,
  detail
}: {
  time: string;
  type: string;
  detail: ReactNode;
}) {
  return (
    <div className="event-entry">
      <time>{time}</time>
      <StatusChip value={type} />
      <span className="event-detail">{detail}</span>
    </div>
  );
}

export function SearchBar({ placeholder = "Search..." }: { placeholder?: string }) {
  return (
    <div className="search-input">
      <Search size={14} strokeWidth={1.5} />
      <input placeholder={placeholder} />
    </div>
  );
}

/* ── Generic UI Primitives ── */

export function Button({
  children,
  className = "",
  disabled,
  onClick,
  type = "button"
}: {
  children: ReactNode;
  className?: string;
  disabled?: boolean;
  onClick?: () => void;
  type?: "button" | "submit" | "reset";
}) {
  return (
    <button
      className={`button ${className}`}
      disabled={disabled}
      onClick={onClick}
      type={type}
    >
      {children}
    </button>
  );
}

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`panel ${className}`}>{children}</div>;
}

export function CardHeader({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={`panel-header ${className}`}>{children}</div>;
}

export function CardTitle({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <h2 className={className}>{children}</h2>;
}

export function CardContent({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <div className={className}>{children}</div>;
}

export function Modal({
  isOpen,
  onClose,
  title,
  children,
  className = "",
  style
}: {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  className?: string;
  style?: React.CSSProperties;
}) {
  if (!isOpen) return null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className={`modal-content ${className}`} style={style} onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h2>{title}</h2>
          <button type="button" onClick={onClose} className="modal-close" aria-label="Close dialog">
            <X size={18} strokeWidth={1.5} />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}
