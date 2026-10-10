"use client";

type Props = { label: string; value: string | number; onChange: (value: string) => void; disabled?: boolean; step?: number; min?: number; max?: number; id?: string };

export default function NumericStepper({ label, value, onChange, disabled = false, step = 1, min = 0, max, id }: Props) {
  const number = Number(value);
  const adjust = (delta: number) => {
    const current = Number.isFinite(number) ? number : min;
    const next = Math.min(max ?? Number.MAX_SAFE_INTEGER, Math.max(min, current + delta));
    onChange(String(next));
  };
  return <div className="flex items-center gap-2">
    <button type="button" aria-label={`Decrease ${label}`} disabled={disabled || (Number.isFinite(number) && number <= min)} onClick={() => adjust(-step)} className="h-10 min-w-10 rounded-lg border border-slate-300 text-lg font-semibold text-slate-700 disabled:opacity-40">−</button>
    <input id={id} aria-label={label} type="number" inputMode="numeric" min={min} max={max} step="1" disabled={disabled} value={value} onChange={event => onChange(event.target.value)} className="h-10 w-24 rounded-lg border border-slate-300 px-3 text-center disabled:bg-slate-100" />
    <button type="button" aria-label={`Increase ${label}`} disabled={disabled || (Number.isFinite(number) && max !== undefined && number >= max)} onClick={() => adjust(step)} className="h-10 min-w-10 rounded-lg border border-slate-300 text-lg font-semibold text-slate-700 disabled:opacity-40">+</button>
  </div>;
}
