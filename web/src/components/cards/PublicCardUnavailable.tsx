"use client";
import Link from "next/link";
import { useCustomerLocale } from "@/lib/customer-locale";

export default function PublicCardUnavailable() {
  const { t } = useCustomerLocale();
  return <main className="grid min-h-screen place-items-center bg-slate-50 px-5 py-16"><section className="w-full max-w-md rounded-3xl border border-slate-200 bg-white p-10 text-center shadow-sm"><p className="text-xs font-bold uppercase tracking-[.2em] text-blue-600">{t("Digital Business Card")}</p><h1 className="mt-3 text-3xl font-bold text-slate-950">{t("Card not available")}</h1><p className="mt-3 text-sm text-slate-500">{t("This card may have moved or is not currently published.")}</p><Link href="/" className="mt-6 inline-block rounded-xl bg-blue-600 px-5 py-3 text-sm font-semibold text-white">{t("Go to home")}</Link></section></main>;
}
