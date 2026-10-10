"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import type { LucideIcon } from "lucide-react";

export default function MetricCard({
  label,
  value,
  Icon,
}: {
  label: string;
  value: number;
  Icon: LucideIcon;
}) {
 const { t, language } = useCustomerLocale();
 

  return (
    <article className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <span className="mb-4 inline-flex rounded-xl bg-blue-50 p-2.5 text-blue-600">
        <Icon size={21} aria-hidden="true" />
      </span>
      <p className="text-3xl font-bold tracking-tight">{value.toLocaleString(language === "ar" ? "ar" : "en")}</p>
      <p className="mt-1 text-sm text-slate-500">{t(label)}</p>
    </article>
  );
}
