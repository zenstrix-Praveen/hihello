"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import Link from "next/link";
import { useState } from "react";
import { ContactRound } from "lucide-react";
import { apiRequest } from "@/lib/user-api";
import { Card, dateLabel } from "./data";

export default function CardTable({ cards, compact = false, onChanged }: { cards: Card[]; compact?: boolean; onChanged?: () => void }) {
  const { t, error: localizeError } = useCustomerLocale();
  const [busy, setBusy] = useState<number | null>(null);
  const [actionError, setActionError] = useState("");
  async function changeStatus(card: Card, status: string) {
    setBusy(card.id); setActionError("");
    try { await apiRequest(`/user/cards/${card.id}/status`, { method: "PATCH", body: JSON.stringify({ status }) }); onChanged?.(); }
    catch (error) { setActionError(error instanceof Error ? error.message : "Could not update card status."); }
    finally { setBusy(null); }
  }
  if (!cards.length) return <div className="p-12 text-center"><ContactRound className="mx-auto mb-4 text-blue-500" size={36} /><h3 className="font-bold">{t("Your next connection starts here")}</h3><p className="mt-2 text-sm text-slate-500">{t("Create your first digital business card and make it yours.")}</p><Link href="/dashboard/cards/new" className="mt-5 inline-block rounded-xl bg-blue-600 px-5 py-3 text-sm font-semibold text-white">{t("Create Card")}</Link></div>;
  const actions = (c: Card) => <div className="flex flex-wrap gap-3 text-xs font-semibold text-blue-600"><Link href={`/dashboard/cards/${c.id}`}>{t("View")}</Link><Link href={`/dashboard/cards/${c.id}/edit`}>{t("Edit")}</Link><Link href={`/dashboard/cards/${c.id}/preview`}>{t("Preview")}</Link>{!compact && <Link href={`/dashboard/analytics?card=${c.id}`}>{t("Analytics")}</Link>}{c.status === "draft" && <button disabled={busy === c.id} onClick={() => changeStatus(c, "published")}>{t("Publish")}</button>}{c.status === "published" && <button disabled={busy === c.id} onClick={() => changeStatus(c, "archived")}>{t("Archive")}</button>}{c.status === "archived" && <button disabled={busy === c.id} onClick={() => changeStatus(c, "draft")}>{t("Restore draft")}</button>}</div>;
  return <>
    {actionError && <p role="alert" className="mx-4 mt-4 rounded-xl bg-red-50 p-3 text-sm text-red-700">{localizeError(actionError)}</p>}
    <div className="space-y-3 p-4 md:hidden">{cards.map(c => <article key={c.id} className="rounded-xl border border-slate-200 p-4"><div className="flex items-start justify-between gap-3"><div><Link className="font-semibold text-slate-900" href={`/dashboard/cards/${c.id}`}>{c.name}</Link><p className="mt-1 text-xs text-slate-500">{t(c.template_name || "—")}{c.company_name ? ` · ${c.company_name}` : ""}</p></div><span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-semibold capitalize">{t(c.status)}</span></div><div className="mt-4 grid grid-cols-3 gap-2 text-xs text-slate-500"><span>{t("Views")}<b className="block text-slate-900">{c.views}</b></span><span>{t("QR")}<b className="block text-slate-900">{c.qr_count}</b></span><span>{t("NFC")}<b className="block text-slate-900">{c.nfc_count}</b></span></div><p className="my-3 text-xs text-slate-500">{t("Updated {date}", { date: dateLabel(c.updated_at) })}</p>{actions(c)}</article>)}</div>
    <div className="hidden overflow-x-auto md:block"><table className="w-full whitespace-nowrap text-start text-sm"><thead className="border-y border-slate-100 bg-slate-50 text-xs uppercase tracking-wide text-slate-500"><tr>{["Card", "Template", ...(!compact ? ["Company"] : []), "Status", "Views", ...(!compact ? ["QR", "NFC"] : []), "Updated", "Actions"].map(x => <th key={x} className="px-5 py-4 font-semibold">{t(x)}</th>)}</tr></thead><tbody className="divide-y divide-slate-100">{cards.map(c => <tr key={c.id} className="hover:bg-slate-50/60"><td className="px-5 py-5"><Link className="font-semibold text-slate-900 hover:text-blue-600" href={`/dashboard/cards/${c.id}`}>{c.name}</Link><p className="mt-1 text-xs text-slate-400">/{c.slug}</p></td><td className="px-5 py-5 text-slate-500">{t(c.template_name || "—")}</td>{!compact && <td className="px-5 py-5">{c.company_name || "—"}</td>}<td className="px-5 py-5"><span className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${c.status === "published" ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-600"}`}>{t(c.status)}</span></td><td className="px-5 py-5">{c.views}</td>{!compact && <><td className="px-5 py-5">{c.qr_count}</td><td className="px-5 py-5">{c.nfc_count}</td></>}<td className="px-5 py-5 text-slate-500">{dateLabel(c.updated_at)}</td><td className="max-w-sm px-5 py-5">{actions(c)}</td></tr>)}</tbody></table></div>
  </>;
}
