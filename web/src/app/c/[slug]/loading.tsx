"use client";
import { useCustomerLocale } from "@/lib/customer-locale";

export default function LoadingPublicCard() {
  const { t } = useCustomerLocale();
  return <main className="grid min-h-screen place-items-center bg-slate-50 px-4 py-8" aria-busy="true">
    <div role="status" className="w-full max-w-xl animate-pulse overflow-hidden rounded-[2rem] border border-slate-200 bg-white shadow-sm">
      <div className="h-44 bg-blue-100" />
      <div className="space-y-4 p-8"><div className="h-4 w-40 rounded bg-slate-100" /><div className="h-9 w-3/4 rounded bg-slate-100" /><div className="h-5 w-1/2 rounded bg-slate-100" /><div className="grid grid-cols-2 gap-3 pt-4"><div className="h-12 rounded-xl bg-slate-100" /><div className="h-12 rounded-xl bg-slate-100" /></div><span className="sr-only">{t("Loading digital business card…")}</span></div>
    </div>
  </main>;
}
