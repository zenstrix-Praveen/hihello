import Link from "next/link";
import { UserRound, Palette, Send, QrCode, Smartphone, ContactRound, BarChart3 } from "lucide-react";
import FeatureIllustration from "./FeatureIllustration";
import { useCustomerLocale } from "@/lib/customer-locale";

const journey = [
  { title: "Create Your Card", text: "Add your name, role and public contact details.", icon: UserRound },
  { title: "Customize Your Design", text: "Choose a template and make it yours.", icon: Palette },
  { title: "Publish", text: "Preview your card, then make it public.", icon: Send },
  { title: "Share QR / NFC / Link", text: "Choose how to share your introduction.", icon: QrCode },
  { title: "Visitor Opens Card", text: "Your public details open on their device.", icon: Smartphone },
  { title: "Save Contact / Connect", text: "They save your details or get in touch.", icon: ContactRound },
  { title: "Track Engagement", text: "Review supported activity in your dashboard.", icon: BarChart3 },
];

export default function FeatureJourney() {
  const { t } = useCustomerLocale();
  return <>
    <section className="mt-20 border-y border-slate-200 py-16" aria-labelledby="journey-heading">
      <p className="text-xs font-bold uppercase tracking-[0.2em] text-blue-700">{t("How Dbizcard Works")}</p>
      <h2 id="journey-heading" className="mt-3 max-w-2xl text-3xl font-black tracking-tight text-slate-900 sm:text-4xl">{t("From introduction to connection in seconds")}</h2>
      <ol className="mt-10 grid gap-6 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7">
        {journey.map(({ title, text, icon: Icon }, index) => <li key={title} className="relative min-w-0">
          <div className="flex items-center gap-3"><span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-blue-100 text-blue-700"><Icon aria-hidden="true" className="h-5 w-5" /></span><span className="text-sm font-bold text-blue-600">{String(index + 1).padStart(2, "0")}</span></div>
          <h3 className="mt-4 font-bold leading-6 text-slate-900">{t(title)}</h3><p className="mt-2 text-sm leading-6 text-slate-600">{t(text)}</p>
        </li>)}
      </ol>
    </section>
    <section className="mt-20 grid items-center gap-8 md:grid-cols-2 lg:gap-16" aria-labelledby="sharing-heading">
      <div><p className="text-xs font-bold uppercase tracking-[0.2em] text-blue-700">{t("One identity. Many ways to share.")}</p><h2 id="sharing-heading" className="mt-4 text-3xl font-black tracking-tight text-slate-900 sm:text-4xl">{t("Share your professional identity anywhere.")}</h2><p className="mt-5 text-lg leading-8 text-slate-600">{t("QR, NFC, email and card links give you multiple ways to connect. From a quick introduction at an event to a thoughtful follow-up, your public card keeps the next step simple.")}</p><Link href="/register" className="mt-6 inline-flex rounded-xl bg-blue-600 px-5 py-3 font-bold text-white hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-4">{t("Create Your Card")} <span aria-hidden="true" className="ml-3">→</span></Link></div>
      <FeatureIllustration src="/features/qr-sharing.webp" alt="A smartphone scanning a QR code to open a public digital business card." />
    </section>
    <section className="mt-20 grid items-center gap-8 md:grid-cols-2 lg:gap-16" aria-labelledby="engagement-heading">
      <div className="order-2 md:order-1"><FeatureIllustration src="/features/analytics.webp" alt="Example engagement dashboard with card views, QR scans, NFC taps and link clicks." /><p className="mt-3 text-center text-xs text-slate-500">{t("Sample numbers for illustration · not live customer data")}</p></div>
      <div className="order-1 md:order-2"><p className="text-xs font-bold uppercase tracking-[0.2em] text-cyan-700">{t("See what happens next")}</p><h2 id="engagement-heading" className="mt-4 text-3xl font-black tracking-tight text-slate-900 sm:text-4xl">{t("Know how your connections engage.")}</h2><p className="mt-5 text-lg leading-8 text-slate-600">{t("Track supported views, scans and interactions from one dashboard. Review recorded card views, QR scans, NFC taps, link clicks and contact interactions when analytics is included in your plan.")}</p><Link href="/register" className="mt-6 inline-flex rounded-xl border border-blue-200 bg-white px-5 py-3 font-bold text-blue-700 hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-4">{t("Start with your card")} <span aria-hidden="true" className="ml-3">→</span></Link></div>
    </section>
  </>;
}
