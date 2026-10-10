"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import { CheckCircle2, Mail, MessageCircle, Phone, Share2, Smartphone } from "lucide-react";
import PublicHeader from "@/components/public/PublicHeader";
import PublicFooter from "@/components/public/PublicFooter";
import HomeProductStory from "@/components/public/HomeProductStory";
import { useWebsiteSection } from "@/components/public/useWebsiteSection";
import { useCustomerLocale } from "@/lib/customer-locale";

const homeDefaults = { hero_title: "Make every professional connection count", hero_subtitle: "Create and share a modern professional profile using a digital business card, QR code and NFC.", primary_button_text: "Create Your Card", primary_button_url: "/register", secondary_button_text: "View Templates", secondary_button_url: "/templates", hero_image_url: "" };
function safeActionHref(value: unknown, fallback: string) {
  if (typeof value !== "string") return fallback;
  const href = value.trim();
  if ((href.startsWith("/") && !href.startsWith("//")) || href.startsWith("#")) return href;
  if (/^(https?:\/\/|mailto:|tel:)/i.test(href)) return href;
  return fallback;
}
function safeImageSource(value: unknown) {
  if (typeof value !== "string") return "";
  const src = value.trim();
  if ((src.startsWith("/") && !src.startsWith("//")) || /^https?:\/\//i.test(src)) return src;
  return "";
}

export default function HomePage() {
  const { t } = useCustomerLocale();
  const cms = useWebsiteSection<any>("home", homeDefaults, { fallbackOnUnavailable: true });
  const [heroImageFailed, setHeroImageFailed] = useState(false);
  const heroImage = safeImageSource(cms.hero_image_url);
  const primaryHref = safeActionHref(cms.primary_button_url, "/register");
  const secondaryHref = safeActionHref(cms.secondary_button_url, "/templates");
  useEffect(() => setHeroImageFailed(false), [heroImage]);
  return <>
    <PublicHeader />
    <main>
      <section className="overflow-hidden bg-[radial-gradient(ellipse_at_78%_32%,rgba(6,182,212,.15),transparent_34%),linear-gradient(135deg,#f8fafc,#eff6ff)]">
        <div className="mx-auto grid min-h-[650px] max-w-7xl items-center gap-12 px-5 py-16 lg:grid-cols-2 lg:px-8">
          <div>
            <p className="text-sm font-bold uppercase tracking-[0.2em] text-blue-600">{t("Digital Business Cards")}</p>
            <h1 className="mt-6 max-w-xl text-5xl font-black leading-tight tracking-tight text-slate-950 lg:text-6xl">{cms.hero_title || t(homeDefaults.hero_title)}</h1>
            <p className="mt-6 max-w-xl text-lg leading-8 text-slate-600">{cms.hero_subtitle || t(homeDefaults.hero_subtitle)}</p>
            <div className="relative z-20 mt-8 flex flex-wrap gap-4">
              <Link href={primaryHref} className="rounded-xl bg-blue-600 px-7 py-3.5 font-semibold text-white hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2">{cms.primary_button_text || t(homeDefaults.primary_button_text)}</Link>
              <Link href={secondaryHref} className="rounded-xl border border-slate-300 bg-white px-7 py-3.5 font-semibold text-slate-800 hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600">{cms.secondary_button_text || t(homeDefaults.secondary_button_text)}</Link>
            </div>
            <div className="mt-10 flex flex-wrap gap-x-8 gap-y-3 text-sm text-slate-500">{["QR Sharing", "NFC Ready", "Contact Exchange"].map(x => <span key={x} className="flex items-center gap-2"><CheckCircle2 aria-hidden="true" className="h-4 w-4 text-emerald-600" />{t(x)}</span>)}</div>
          </div>
          <div className="flex justify-center">
            {heroImage && !heroImageFailed && <img src={heroImage} alt="Dbizcard homepage hero illustration" onError={() => setHeroImageFailed(true)} className="max-h-[600px] w-full max-w-[520px] rounded-3xl object-cover shadow-xl"/>}
            <div className={`${heroImage && !heroImageFailed ? "hidden" : ""} w-full max-w-[390px] rounded-[3rem] bg-slate-950 p-3 shadow-[0_35px_90px_-36px_rgba(15,23,42,.55)]`}>
              <div className="relative overflow-hidden rounded-[2.45rem] bg-white"><div className="absolute left-1/2 top-0 h-7 w-36 -translate-x-1/2 rounded-b-3xl bg-slate-950" />
                <div className="bg-gradient-to-br from-blue-700 via-blue-600 to-cyan-500 px-7 pb-8 pt-14 text-white"><div className="mx-auto flex h-24 w-24 items-center justify-center rounded-full border-4 border-white/80 bg-white text-3xl font-black text-blue-700 shadow-lg">DC</div><div className="mt-5 text-center"><h2 className="text-2xl font-bold">{t("Your Name")}</h2><p className="mt-1 text-sm text-blue-100">{t("Founder · Your Company")}</p></div></div>
                <div className="grid grid-cols-4 gap-3 border-b px-5 py-5">{[{icon:Phone,label:"Call"},{icon:MessageCircle,label:"WhatsApp"},{icon:Mail,label:"Email"},{icon:Share2,label:"Share"}].map(({icon:Icon,label})=><div key={label} className="flex flex-col items-center gap-2 text-xs font-medium text-slate-600"><span className="flex h-11 w-11 items-center justify-center rounded-xl bg-blue-50 text-blue-600"><Icon aria-hidden="true" className="h-5 w-5" /></span>{t(label)}</div>)}</div>
                <div className="space-y-4 p-6"><div><p className="text-xs font-semibold uppercase tracking-wide text-slate-400">{t("Phone")}</p><p className="mt-1 font-medium text-slate-800">{t("Your phone number")}</p></div><div><p className="text-xs font-semibold uppercase tracking-wide text-slate-400">{t("Email")}</p><p className="mt-1 font-medium text-slate-800">you@example.com</p></div><div><p className="text-xs font-semibold uppercase tracking-wide text-slate-400">{t("Website")}</p><p className="mt-1 font-medium text-blue-600">yourwebsite.com</p></div><Link href="/register" className="block w-full rounded-xl bg-blue-600 px-4 py-3 text-center font-semibold text-white">{t("Create Your Card")}</Link></div>
              </div>
            </div>
          </div>
        </div>
      </section>
      <HomeProductStory />
    </main>
    <PublicFooter />
  </>;
}
