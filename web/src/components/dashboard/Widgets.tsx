"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import Link from "next/link";
import { ContactRound, Eye, FilePenLine, QrCode, SmartphoneNfc, Users } from "lucide-react";
import { Summary } from "./data";
import MetricCard from "./MetricCard";
export function LoadState({ error, loading, retry }: { error: string; loading: boolean; retry: () => void }) {
 const { t, error: localizeError } = useCustomerLocale();
 

  if (loading) return <p role="status" className="rounded-2xl border border-slate-200 bg-white p-8 text-slate-500">{t("Loading your data…")}</p>;
  if (error) return <div role="alert" className="rounded-2xl border border-red-200 bg-red-50 p-6 text-red-800">{localizeError(error)}<button onClick={retry} className="ms-4 font-bold underline">{t("Try again")}</button></div>;
  return null;
}
export function Metrics({ data }: { data: Summary }) {
 const { t } = useCustomerLocale();
 

  const metrics = [["Total Cards", data.total_cards, ContactRound], ["Published Cards", data.published_cards, Eye], ["Draft Cards", data.draft_cards, FilePenLine], ["QR Scans", data.total_qr_scans, QrCode], ["NFC Taps", data.total_nfc_taps, SmartphoneNfc], ["Contacts", data.total_contacts, Users]] as const;
  return <div className="grid grid-cols-2 gap-4 xl:grid-cols-3 2xl:grid-cols-6">{metrics.map(([label, value, Icon]) => <MetricCard key={label} label={t(label)} value={value} Icon={Icon} />)}</div>;
}
export function CurrentPlan({ plan }: { plan: Summary["current_plan"] }) {
 const { t, language } = useCustomerLocale();
 const customerDate=(value:string|null)=>value?new Date(value).toLocaleDateString(language === "ar" ? "ar" : "en",{year:"numeric",month:"short",day:"numeric"}):"—";

  return <section className="rounded-2xl border border-slate-200 bg-white p-6"><div className="flex items-center justify-between gap-4"><h2 className="text-lg font-bold">{t("Current Plan")}</h2><Link href="/dashboard/billing" className="text-sm font-semibold text-blue-600">{t("Manage billing →")}</Link></div>{plan ? <div className="mt-5 grid gap-5 sm:grid-cols-4">{[["Plan", plan.name], ["Status", t(plan.status)], ["Billing period", t(plan.billing_period)], ["Renewal", plan.renews_at ? customerDate(plan.renews_at) : t("No scheduled renewal")]].map(([label, value]) => <div key={label}><p className="text-xs uppercase tracking-wide text-slate-400">{t(label)}</p><p className="mt-1 font-semibold capitalize">{value}</p></div>)}</div> : <div className="mt-4"><p className="text-slate-600">{t("You don’t have a current subscription.")}</p><Link href="/dashboard/billing" className="mt-3 inline-block text-sm font-semibold text-blue-600">{t("Explore plans →")}</Link></div>}</section>;
}
