import type { ReactNode } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";

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
  title?: string;
  subtitle?: string;
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
  tone = "neutral"
}: {
  label: string;
  value: string | number;
  detail?: string;
  tone?: Tone;
}) {
  return (
    <div className={`stat-card tone-${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      {detail && <small>{detail}</small>}
    </div>
  );
}

export function StatusChip({ value }: { value?: string }) {
  const normalized = value || "unknown";
  let tone: Tone = "neutral";
  if (["online", "acked", "enabled", "healthy"].includes(normalized.toLowerCase())) tone = "success";
  if (["warning", "pending"].includes(normalized.toLowerCase())) tone = "warning";
  if (["critical", "nacked", "disabled", "error"].includes(normalized.toLowerCase())) tone = "danger";
  if (["ota"].includes(normalized.toLowerCase())) tone = "info";
  return <span className={`status-chip tone-${tone}`}>{normalized}</span>;
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
      <AlertTriangle size={20} aria-hidden />
      <h3>{title}</h3>
      <p>{description}</p>
      {action}
    </div>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="empty-state error">
      <AlertTriangle size={20} aria-hidden />
      <h3>Unable to load data</h3>
      <p>{message}</p>
      {onRetry && (
        <button className="button secondary" type="button" onClick={onRetry}>
          <RefreshCw size={15} aria-hidden />
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
