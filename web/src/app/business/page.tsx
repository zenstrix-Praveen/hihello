"use client";
import Link from "next/link";
import { useState } from "react";
import {
  Building2,
  Users,
  ShieldCheck,
  BarChart3,
  Palette,
  QrCode,
  Nfc,
  CreditCard,
  CheckCircle2,
  type LucideIcon,
} from "lucide-react";

import PublicHeader from "@/components/public/PublicHeader";
import PublicFooter from "@/components/public/PublicFooter";
import { useWebsiteSection } from "@/components/public/useWebsiteSection";
import { useCustomerLocale } from "@/lib/customer-locale";

const useCases = [
  "Sales teams",
  "Corporate employees",
  "Recruitment teams",
  "Consultants",
  "Agencies",
  "Event teams",
  "Customer-facing staff",
  "Large organizations",
];

export default function BusinessPage() {
  const {t}=useCustomerLocale();
  const cms = useWebsiteSection<any>("business", { page_title: "Business & Enterprise", hero_heading: "Digital business cards built for growing teams", hero_subheading: "Manage professional digital identities for your entire company with centralized branding, team control, QR, NFC and analytics.", description: "Give every team member a professional digital business card while keeping company branding and administration under control.", benefits: [], cta_heading: "Ready to bring your team online?", cta_text: "Talk to us about team plans, company branding, centralized management and larger digital-card deployments.", cta_button_label: "Contact Sales", cta_url: "/contact", image_url: "", enabled: true }, { fallbackOnUnavailable: true });
  const [imageFailed, setImageFailed] = useState(false);
  const visibleBenefits: { icon: LucideIcon; title: string; description: string }[] = Array.isArray(cms.benefits)
    ? cms.benefits.map((text: string, i: number) => ({ icon: CheckCircle2, title: `Benefit ${i + 1}`, description: text }))
    : [];
  if (cms.enabled === false) return <><PublicHeader /><PublicFooter /></>;
  return (
    <>
      <PublicHeader />

      <main>
        <section className="overflow-hidden bg-slate-950 text-white">
          <div className="mx-auto grid max-w-7xl items-center gap-12 px-5 py-24 lg:grid-cols-2 lg:px-8">
            <div>
              <div className="inline-flex items-center gap-2 rounded-full border border-blue-400/20 bg-blue-500/10 px-4 py-2 text-sm font-semibold text-blue-300">
                <Building2 className="h-4 w-4" />
                {cms.page_title || t("Business & Teams")}
              </div>

              <h1 className="mt-6 text-5xl font-black leading-tight sm:text-6xl">
                {cms.hero_heading}
              </h1>

              <p className="mt-6 max-w-xl text-lg leading-8 text-slate-300">
                {cms.hero_subheading || cms.description}
              </p>

              <div className="mt-8 flex flex-wrap gap-4">
                <Link
                  href={cms.cta_url || "/contact"}
                  className="rounded-xl bg-blue-600 px-7 py-3.5 font-bold text-white transition hover:bg-blue-500"
                >
                  {cms.cta_button_label || cms.cta_text || t("Contact Sales")}
                </Link>

                <Link
                  href="/pricing"
                  className="rounded-xl border border-white/20 px-7 py-3.5 font-bold text-white transition hover:bg-white/10"
                >
                  {t("View Plans")}
                </Link>
              </div>
            </div>

            {cms.image_url && !imageFailed ? <img src={cms.image_url} onError={() => setImageFailed(true)} alt="Business and enterprise digital card illustration" className="max-h-[520px] w-full rounded-3xl object-cover"/> : <div className="rounded-3xl border border-white/10 bg-white/5 p-8 backdrop-blur">
              <div className="grid gap-5 sm:grid-cols-2">
                <div className="rounded-2xl bg-white p-6 text-slate-900">
                  <Users className="h-8 w-8 text-blue-600" />
                  <p className="mt-4 text-3xl font-black">100+</p>
                  <p className="mt-1 text-sm text-slate-500">
                    {t("Team members supported")}
                  </p>
                </div>

                <div className="rounded-2xl bg-white p-6 text-slate-900">
                  <QrCode className="h-8 w-8 text-blue-600" />
                  <p className="mt-4 text-3xl font-black">{t("QR + NFC")}</p>
                  <p className="mt-1 text-sm text-slate-500">
                    {t("Multiple sharing methods")}
                  </p>
                </div>

                <div className="rounded-2xl bg-white p-6 text-slate-900">
                  <BarChart3 className="h-8 w-8 text-blue-600" />
                  <p className="mt-4 text-3xl font-black">{t("Live")}</p>
                  <p className="mt-1 text-sm text-slate-500">
                    {t("Engagement analytics")}
                  </p>
                </div>

                <div className="rounded-2xl bg-white p-6 text-slate-900">
                  <ShieldCheck className="h-8 w-8 text-blue-600" />
                  <p className="mt-4 text-3xl font-black">{t("Central")}</p>
                  <p className="mt-1 text-sm text-slate-500">
                    {t("Business administration")}
                  </p>
                </div>
              </div>
            </div>}
          </div>
        </section>

        <section className="bg-white py-20">
          <div className="mx-auto max-w-7xl px-5 lg:px-8">
            <div className="mx-auto max-w-3xl text-center">
              <p className="text-sm font-bold uppercase tracking-[0.2em] text-blue-600">
                {t("Built for Organizations")}
              </p>

              <h2 className="mt-3 text-4xl font-black text-slate-950">
                {t("Everything your business needs")}
              </h2>

              <p className="mt-4 text-slate-500">
                {cms.description}
              </p>
            </div>

            <div className="mt-12 grid gap-6 md:grid-cols-2 lg:grid-cols-3">
              {visibleBenefits.map(({ icon: Icon, title, description }) => (
                <div
                  key={title}
                  className="rounded-3xl border bg-white p-7 shadow-sm transition hover:-translate-y-1 hover:shadow-lg"
                >
                  <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-blue-50 text-blue-600">
                    <Icon className="h-6 w-6" />
                  </div>

                  <h3 className="mt-5 text-xl font-bold text-slate-900">
                    {t(title)}
                  </h3>

                  <p className="mt-3 leading-7 text-slate-500">
                    {description}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="bg-slate-50 py-20">
          <div className="mx-auto grid max-w-7xl gap-12 px-5 lg:grid-cols-2 lg:px-8">
            <div>
              <p className="text-sm font-bold uppercase tracking-[0.2em] text-blue-600">
                {t("Use Cases")}
              </p>

              <h2 className="mt-3 text-4xl font-black text-slate-950">
                {t("Suitable for teams of every kind")}
              </h2>

              <p className="mt-4 max-w-xl leading-7 text-slate-500">
                {t("Whether you are managing a small team or a larger organization, digital cards help standardize professional identity and make sharing easier.")}
              </p>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              {useCases.map((item) => (
                <div
                  key={item}
                  className="flex items-center gap-3 rounded-2xl border bg-white p-4"
                >
                  <CheckCircle2 className="h-5 w-5 shrink-0 text-green-600" />
                  <span className="font-semibold text-slate-800">{t(item)}</span>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="bg-white py-20">
          <div className="mx-auto max-w-5xl px-5">
            <div className="rounded-3xl bg-blue-600 p-10 text-center text-white sm:p-14">
              <CreditCard className="mx-auto h-11 w-11" />

              <h2 className="mt-5 text-4xl font-black">
                {cms.cta_heading || t("Ready to bring your team online?")}
              </h2>

              <p className="mx-auto mt-4 max-w-2xl text-blue-100">
                {cms.cta_text || t("Talk to us about team plans, company branding, centralized management and larger digital-card deployments.")}
              </p>

              {cms.contact_sales_email && <p className="mt-4 text-blue-100"><a href={`mailto:${cms.contact_sales_email}`}>{cms.contact_sales_email}</a></p>}
              <div className="mt-8 flex flex-wrap justify-center gap-4">
                <Link
                  href={cms.cta_url || "/contact"}
                  className="rounded-xl bg-white px-7 py-3.5 font-bold text-blue-700"
                >
                  {cms.cta_button_label || cms.cta_text || t("Contact Sales")}
                </Link>

                <Link
                  href="/pricing"
                  className="rounded-xl border border-white/30 px-7 py-3.5 font-bold text-white"
                >
                  {t("Explore Pricing")}
                </Link>
              </div>
            </div>
          </div>
        </section>
      </main>

      <PublicFooter />
    </>
  );
}
