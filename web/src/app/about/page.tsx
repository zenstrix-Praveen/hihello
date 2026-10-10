"use client";
import PublicHeader from "@/components/public/PublicHeader";
import PublicFooter from "@/components/public/PublicFooter";
import { useCustomerLocale } from "@/lib/customer-locale";

export default function AboutPage() {
  const { t } = useCustomerLocale();
  return <><PublicHeader/><main className="mx-auto max-w-5xl px-5 py-16 sm:py-24"><p className="text-sm font-bold uppercase tracking-widest text-blue-700">{t("About Dbizcard")}</p><h1 className="mt-3 text-4xl font-black text-slate-950">{t("A simpler way to share your professional identity")}</h1><p className="mt-6 max-w-3xl text-lg leading-8 text-slate-600">{t("Dbizcard helps people and teams create, manage and share digital business cards. Keep your public contact information together and make it easy for others to connect.")}</p><section className="mt-12 grid gap-5 md:grid-cols-3">{[["Create","Build a digital card with your professional details and a design that fits your identity."],["Share","Share your public card using its link, QR code or configured NFC tag."],["Connect","Give visitors clear options to call, email, visit your website or save your public contact details."]].map(([title,description])=><article key={title} className="rounded-2xl border border-slate-200 p-6"><h2 className="text-xl font-bold">{t(title)}</h2><p className="mt-3 leading-7 text-slate-600">{t(description)}</p></article>)}</section></main><PublicFooter/></>;
}
