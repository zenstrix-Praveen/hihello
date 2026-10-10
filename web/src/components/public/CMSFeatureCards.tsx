"use client";

import { ArrowUpRight, Sparkles } from "lucide-react";
import { useCustomerLocale } from "@/lib/customer-locale";


type Item = { id: number; title: string; description: string; detailed_description: string; icon_key: string; illustration_url: string; cta_label: string; cta_url: string; sort_order: number; is_enabled: boolean };
export default function CMSFeatureCards({ content }: { content: Record<string, any> }) {
  const { t }=useCustomerLocale();
  const items: Item[] = Array.isArray(content.items) ? content.items.filter((item: Item) => item.is_enabled !== false).sort((a: Item, b: Item) => a.sort_order - b.sort_order) : [];
  if (content.enabled === false) return null;
  if (!items.length) return <p className="py-8 text-center text-slate-500">{t("No items found")}</p>;
  return <section className="mt-8" aria-label={t("Featured product capabilities")}><div className="mb-6 text-center"><p className="text-xs font-extrabold uppercase tracking-[.2em] text-blue-700">{t("From Dbizcard")}</p><h2 className="mt-3 text-2xl font-black tracking-tight text-slate-900">{t("Featured capabilities")}</h2></div><div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">{items.map(item => <article key={item.id} className="overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-sm"><div className="grid h-44 place-items-center bg-gradient-to-br from-blue-50 via-white to-indigo-100">{item.illustration_url ? <img src={item.illustration_url} alt="" loading="lazy" onError={event => { event.currentTarget.style.display = "none"; }} className="h-full w-full object-contain p-4"/> : <Sparkles aria-hidden="true" className="h-12 w-12 text-blue-500"/>}</div><div className="p-5"><p className="text-xs font-bold uppercase tracking-wider text-blue-700">{t(item.icon_key || "Dbizcard Feature")}</p><h3 className="mt-2 text-lg font-bold text-slate-900">{item.title}</h3><p className="mt-2 text-sm leading-6 text-slate-600">{item.description}</p>{item.detailed_description && <details className="mt-3"><summary className="cursor-pointer text-sm font-semibold text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">{t("How it works")}</summary><p className="mt-2 text-sm leading-6 text-slate-600">{item.detailed_description}</p></details>}{item.cta_label && item.cta_url && <a href={item.cta_url} className="mt-4 inline-flex items-center gap-1 text-sm font-bold text-blue-700 hover:text-blue-900">{item.cta_label}<ArrowUpRight aria-hidden="true" className="h-4 w-4"/></a>}</div></article>)}</div></section>;
}
