"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import Link from "next/link";
import { useState } from "react";
import { Card, useUserData } from "./data";
import { LoadState } from "./Widgets";
import CardEditor from "./CardEditor";
import CardPreview from "@/components/cards/CardPreview";
import { apiRequest } from "@/lib/user-api";
import CardSharePanel from "./CardSharePanel";
export default function CardDetail({ id, mode = "view" }: { id: string; mode?: "view" | "edit" | "preview" }) {
  const { t, error: localizeError } = useCustomerLocale();
  const state = useUserData<{ data: Card }>(`/user/cards/${encodeURIComponent(id)}`);
  const [error, setError] = useState(""); const [saving, setSaving] = useState(false);
  if (!state.data) return <LoadState {...state} retry={state.reload} />;
  const card = state.data.data;
  if (mode === "edit") return <CardEditor card={card} />;
  async function changeStatus(status: string) {
    setSaving(true); setError("");
    try { await apiRequest(`/user/cards/${id}/status`, { method: "PATCH", body: JSON.stringify({ status }) }); state.reload(); }
    catch(e) { setError(e instanceof Error ? e.message : "Could not change status"); } finally { setSaving(false); }
  }
  return <div className="space-y-6"><Link href="/dashboard/cards" className="text-sm font-semibold text-blue-600">{t("← My Digital Cards")}</Link><div className="flex flex-wrap items-center justify-between gap-4"><div><p className="text-xs font-bold uppercase tracking-[.18em] text-blue-700">{mode === "preview" ? t("Card created") : t("My card")}</p><h1 className="mt-2 text-3xl font-bold">{mode === "preview" ? t("✓ Your digital card is ready") : card.name}</h1><p className="mt-2 text-sm text-slate-500"><bdi dir="ltr">/{card.slug}</bdi> · {t(card.status)}</p></div><div className="flex flex-wrap gap-4 text-sm font-semibold text-blue-600"><Link href={`/dashboard/cards/${id}/edit`}>{t("Edit Card")}</Link><Link href={`/dashboard/analytics?card=${id}`}>{t("Analytics")}</Link><Link href="/dashboard/cards">{t("Go to My Cards")}</Link></div></div>
    {mode === "preview" ? <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(360px,0.8fr)]"><section className="rounded-3xl border border-slate-200 bg-gradient-to-b from-blue-50 to-white p-5 shadow-lg shadow-slate-900/5 sm:p-8"><div className="mb-5 flex flex-wrap items-center justify-between gap-3"><div><h2 className="font-bold">{t("Card preview")}</h2><p className="mt-1 text-sm text-slate-500">{t("{template} · final design", { template: t(card.template_name || "Dbizcard template") })}</p></div><span className={`rounded-full px-3 py-1 text-xs font-bold capitalize ${card.status === "published" ? "bg-emerald-50 text-emerald-800" : "bg-amber-50 text-amber-900"}`}>{t(card.status)}</span></div><div className="mx-auto max-w-sm rounded-[2.5rem] border-[8px] border-slate-900 bg-slate-900 p-2 shadow-2xl"><div className="overflow-hidden rounded-[2rem] bg-white"><CardPreview card={card}/></div></div><div className="mt-6 flex flex-wrap justify-center gap-3"><a href={card.status === "published" ? `/c/${encodeURIComponent(card.slug)}` : undefined} target="_blank" rel="noreferrer" aria-disabled={card.status !== "published"} className={`rounded-xl px-5 py-3 text-sm font-semibold text-white ${card.status === "published" ? "bg-blue-600 hover:bg-blue-700" : "pointer-events-none bg-slate-400"}`}>{t("Open Public Card")}</a><Link href={`/dashboard/cards/${id}/edit`} className="rounded-xl border border-blue-200 bg-white px-5 py-3 text-sm font-semibold text-blue-800">{t("Edit Card")}</Link><a href="#share-card" className="rounded-xl border border-slate-200 bg-white px-5 py-3 text-sm font-semibold text-slate-800">{t("Share Card")}</a></div></section><div id="share-card"><CardSharePanel card={card}/></div></div> : <div className="mx-auto max-w-lg"><CardPreview card={card} /></div>}
    {mode === "view" && <section className="rounded-2xl border border-slate-200 bg-white p-6"><h2 className="font-bold">{t("Publishing")}</h2><p className="mt-2 text-sm text-slate-500">{t("Current status: {status}", { status: t(card.status) })}</p><div className="mt-4 flex flex-wrap gap-3">{["draft", "published", "archived"].map(s => <button key={s} disabled={saving || card.status === s || card.status === "suspended"} onClick={() => changeStatus(s)} className="rounded-lg border border-slate-200 px-4 py-2 text-sm font-semibold capitalize disabled:opacity-40">{t(s)}</button>)}</div>{error && <p role="alert" className="mt-3 text-red-600">{localizeError(error)}</p>}</section>}
  </div>;
}
