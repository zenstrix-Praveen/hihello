"use client";
import PublicHeader from "@/components/public/PublicHeader";
import PublicFooter from "@/components/public/PublicFooter";
import Link from "next/link";
import { ArrowRight, Sparkles } from "lucide-react";
import CMSFeatureCards from "@/components/public/CMSFeatureCards";
import { useWebsiteSection } from "@/components/public/useWebsiteSection";
import { featuresPageDefaults } from "@/lib/features-page-content";
import { useCustomerLocale } from "@/lib/customer-locale";

export default function FeaturesPage() {
  const { t } = useCustomerLocale();
  const cms = useWebsiteSection<any>("features", featuresPageDefaults, { fallbackOnUnavailable: true });
  if (cms.enabled === false) return <><PublicHeader /><PublicFooter /></>;
  return (
    <>
      <PublicHeader dbizcardBrand />

      <section className="relative isolate flex min-h-[300px] items-center overflow-hidden bg-[linear-gradient(135deg,#0F172A_0%,#172554_45%,#1E3A8A_100%)] py-16 text-white sm:min-h-[340px] sm:py-20">
        <div aria-hidden="true" className="absolute -right-24 -top-36 -z-10 h-[28rem] w-[28rem] rounded-full bg-blue-500/20 blur-3xl" />
        <div aria-hidden="true" className="absolute -bottom-48 left-[18%] -z-10 h-[26rem] w-[26rem] rounded-full bg-cyan-400/15 blur-3xl" />
        <div aria-hidden="true" className="absolute inset-0 -z-10 bg-[radial-gradient(ellipse_at_75%_20%,rgba(59,130,246,0.22),transparent_42%)]" />
        <div className="mx-auto w-full max-w-7xl px-5 sm:px-6 lg:px-8">
          <div className="max-w-5xl">
            <div className="inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/10 px-3.5 py-1.5 text-xs font-bold tracking-[0.16em] text-blue-100 shadow-sm backdrop-blur-sm">
              <Sparkles aria-hidden="true" className="h-4 w-4 text-cyan-300" />
              {t("EXPLORE FEATURES")}
            </div>
            <h1 className="mt-6 text-4xl font-black tracking-tight sm:text-5xl lg:text-6xl">
              {(cms.page_heading || t("Everything Your Digital Card Needs"))}
            </h1>
            <p className="mt-5 max-w-2xl text-base leading-7 text-blue-100/90 sm:text-lg sm:leading-8">
              {(cms.page_subtitle || t("Everything you need to create, share and grow your professional digital identity."))}
            </p>
            <nav aria-label="Breadcrumb" className="mt-7 text-sm text-blue-100/75">
              <Link href="/" className="transition hover:text-white focus-visible:rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-300">{t("Home")}</Link>
              <span aria-hidden="true" className="mx-2 text-blue-200/60">/</span>
              <span aria-current="page" className="font-medium text-white">{t("Features")}</span>
            </nav>
          </div>
        </div>
      </section>

      <main className="bg-[radial-gradient(ellipse_at_12%_12%,rgba(59,130,246,0.08),transparent_28%),radial-gradient(ellipse_at_88%_55%,rgba(79,70,229,0.06),transparent_32%),#F8FAFC] py-14 sm:py-20 lg:py-24">
        <div className="mx-auto max-w-[1600px] px-4 sm:px-6 lg:px-8">
          <div className="text-center">
            <p className="text-xs font-extrabold uppercase tracking-[0.22em] text-blue-700 sm:text-sm">
              {t("Powerful Features")}
            </p>
            <h2 className="mt-3 text-3xl font-black tracking-tight text-slate-900 sm:text-4xl">
              {t("Built for Modern Networking")}
            </h2>
            <p className="mx-auto mt-4 max-w-[700px] text-base leading-7 text-slate-600 sm:text-lg">
              {t("Create, share and manage your professional identity with tools designed to make every connection easier.")}
            </p>
          </div>
          <CMSFeatureCards content={cms} />
          <section className="relative mt-16 overflow-hidden rounded-[30px] bg-gradient-to-br from-slate-950 via-blue-950 to-blue-700 px-6 py-10 text-white shadow-[0_24px_60px_-30px_rgba(37,99,235,0.7)] sm:mt-24 sm:px-10 sm:py-12 lg:px-14 lg:py-14">
            <div aria-hidden="true" className="absolute -right-20 -top-32 h-80 w-80 rounded-full bg-cyan-300/20 blur-3xl" />
            <div aria-hidden="true" className="absolute -bottom-44 left-1/3 h-80 w-80 rounded-full bg-blue-300/20 blur-3xl" />
            <div className="relative flex flex-col items-start justify-between gap-8 lg:flex-row lg:items-center">
              <div className="max-w-2xl">
                <p className="text-sm font-bold uppercase tracking-[0.16em] text-blue-100">{t("Make your next connection count")}</p>
                <h2 className="mt-3 text-2xl font-black tracking-tight sm:text-3xl lg:text-4xl">{t("Ready to make every introduction count?")}</h2>
                <p className="mt-4 max-w-xl leading-7 text-blue-50/90">
                  {t("Create your digital business card and start sharing today.")}
                </p>
              </div>
              <div className="flex w-full flex-col gap-3 sm:w-auto sm:flex-row lg:shrink-0">
                <Link href="/register" className="inline-flex items-center justify-center gap-2 rounded-xl bg-white px-5 py-3.5 font-bold text-blue-700 shadow-lg transition hover:-translate-y-0.5 hover:bg-blue-50 motion-reduce:transform-none motion-reduce:transition-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white focus-visible:ring-offset-2 focus-visible:ring-offset-indigo-600">
                  {t("Create Your Card")} <ArrowRight aria-hidden="true" className="h-4 w-4" />
                </Link>
                <Link href="/templates" className="inline-flex items-center justify-center rounded-xl border border-white/35 bg-white/10 px-5 py-3.5 font-bold text-white backdrop-blur-sm transition hover:bg-white/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white">
                  {t("Explore Templates")}
                </Link>
              </div>
            </div>
          </section>
        </div>
      </main>

      <PublicFooter />
    </>
  );
}
