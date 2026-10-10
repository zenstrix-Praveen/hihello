"use client";
import PublicHeader from "@/components/public/PublicHeader";
import PublicFooter from "@/components/public/PublicFooter";
import { useCustomerLocale } from "@/lib/customer-locale";

const questions = [
  ["What is a digital business card?", "A digital business card is a public online profile that can include your professional contact details, company information and useful links."],
  ["How do I share my card?", "Share the public card link, show its QR code, or use a compatible NFC card or tag configured with the link."],
  ["Can I update my card after publishing?", "Yes. Edit your card in your workspace. Your public card reflects the saved information you choose to publish."],
  ["Can visitors save my contact details?", "Visitors can use Save Contact to open or download a vCard containing supported public details."],
  ["Do NFC and QR codes work on every phone?", "Device and browser support can vary. QR codes can be scanned with a compatible camera or reader, and NFC requires compatible hardware and settings."],
  ["Where can I manage my subscription?", "Open Plans & Billing in your customer workspace to review plans, subscription details and payment history."],
];

export default function FAQPage() {
  const { t } = useCustomerLocale();
  return <><PublicHeader/><main className="mx-auto max-w-4xl px-5 py-16 sm:py-24"><p className="text-sm font-bold uppercase tracking-widest text-blue-700">{t("Help")}</p><h1 className="mt-3 text-4xl font-black text-slate-950">{t("Frequently Asked Questions")}</h1><p className="mt-4 text-lg text-slate-600">{t("Answers to common questions about digital cards, sharing and billing.")}</p><div className="mt-10 divide-y divide-slate-200 rounded-2xl border border-slate-200 bg-white px-6">{questions.map(([question,answer])=><details key={question} className="py-5"><summary className="cursor-pointer font-bold text-slate-900">{t(question)}</summary><p className="mt-3 leading-7 text-slate-600">{t(answer)}</p></details>)}</div></main><PublicFooter/></>;
}
