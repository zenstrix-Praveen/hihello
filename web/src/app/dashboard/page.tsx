"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import Link from "next/link";
import { Plus, ArrowUpRight } from "lucide-react";
import { useUser } from "@/components/dashboard/UserSession";
import { CardList, Summary, useUserData } from "@/components/dashboard/data";
import { CurrentPlan, LoadState, Metrics } from "@/components/dashboard/Widgets";
import CardTable from "@/components/dashboard/CardTable";
export default function DashboardPage() {
 const { t } = useCustomerLocale();
 

  const user = useUser(); const summary = useUserData<Summary>("/user/dashboard"); const cards = useUserData<CardList>("/user/cards?limit=5");
  return <div className="space-y-7"><div className="flex flex-wrap items-center justify-between gap-4"><div><p className="mb-2 text-xs font-bold uppercase tracking-[.2em] text-blue-600">{t("Overview")}</p><h1 className="text-3xl font-bold tracking-tight">{t("Welcome,")} <bdi>{user.name}</bdi></h1><p className="mt-2 text-sm text-slate-500">{t("Your cards, connections, and activity. All in one place.")}</p></div><Link href="/dashboard/cards/new" className="flex items-center gap-2 rounded-xl bg-blue-600 px-5 py-3 text-sm font-semibold text-white shadow-sm hover:bg-blue-700"><Plus size={18} />{t("Create Card")}</Link></div>
    <LoadState {...summary} retry={summary.reload} />{summary.data && <Metrics data={summary.data} />}
    <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white"><div className="flex items-center justify-between p-6"><h2 className="text-lg font-bold">{t("Recent Digital Cards")}</h2><Link href="/dashboard/cards" className="text-sm font-semibold text-blue-600">{t("View all →")}</Link></div><LoadState {...cards} retry={cards.reload} />{cards.data && <CardTable cards={cards.data.data} compact />}</section>
    {summary.data && <CurrentPlan plan={summary.data.current_plan} />}
    <section><h2 className="mb-4 text-lg font-bold">{t("Quick actions")}</h2><div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">{[["Create Card", "Build your digital introduction", "/dashboard/cards/new"], ["Browse Templates", "Find a design that feels like you", "/dashboard/templates"], ["View Analytics", "See how people connect with you", "/dashboard/analytics"], ["Manage Billing", "Your plan and payment history", "/dashboard/billing"]].map(([title, text, href]) => <Link key={href} href={href} className="rounded-2xl border border-slate-200 bg-white p-5 hover:border-blue-300"><div className="flex items-center justify-between font-semibold">{t(title)}<ArrowUpRight size={18} className="text-blue-500" /></div><p className="mt-2 text-sm text-slate-500">{t(text)}</p></Link>)}</div></section>
  </div>;
}
