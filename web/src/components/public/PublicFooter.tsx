"use client";
import Link from "next/link";
import { ArrowRight, CreditCard } from "lucide-react";
import type { ReactNode } from "react";
import { useWebsiteSection } from "@/components/public/useWebsiteSection";
import BrandLogo from "@/components/brand/BrandLogo";
import { useCustomerLocale } from "@/lib/customer-locale";

const defaultLinkGroups = [
  {
    title: "Product",
    links: [
      { label: "Features", href: "/features" },
      { label: "Templates", href: "/templates" },
      { label: "Pricing", href: "/pricing" },
    ],
  },
  {
    title: "Company",
    links: [
      { label: "Business", href: "/business" },
      { label: "Contact", href: "/contact" },
    ],
  },
  {
    title: "Account",
    links: [
      { label: "Customer Login", href: "/login" },
      { label: "Administrator Login", href: "/admin/login" },
      { label: "Create Account", href: "/register" },
    ],
  },
];

export default function PublicFooter() {
  const { t } = useCustomerLocale();
  const cmsText = (value: unknown) => String(value || "").replace(/hihello/gi, "DBIZCARD");
  const cms = useWebsiteSection<any>("footer", { brand_name: "DBIZCARD", logo_url: "", company_description: "Digital business cards for modern professionals. Create, share and manage your professional identity from one place.", link_groups: null, social_links: [], copyright_text: "", cta_heading: "Ready to create your digital business card?", cta_description: "Build your professional identity and share it anywhere in seconds.", cta_primary_text: "Get Started Free", cta_primary_url: "/register", cta_secondary_text: "View Templates", cta_secondary_url: "/templates", enabled: true });
  const groups: { title: string; links: { label: string; href: string }[] }[] = Array.isArray(cms.link_groups) ? cms.link_groups.filter((g: any) => g.enabled !== false).map((g: any) => ({ title: cmsText(g.title), links: (g.links || []).filter((l: any) => l.enabled !== false).map((l: any) => ({ label: cmsText(l.label), href: String(l.url || "") })) })) : defaultLinkGroups;
  const copyright = cmsText(cms.copyright_text || "© {year} DBIZCARD. All rights reserved.").replace("{year}", String(new Date().getFullYear())).replace(/dbizcard/gi, "DBIZCARD");
  const link = (href: string, label: ReactNode, className?: string) => href.startsWith("/") && !href.startsWith("//") || href.startsWith("#") ? <Link href={href} className={className}>{label}</Link> : <a href={href} rel="noopener noreferrer" className={className}>{label}</a>;
  if (cms.enabled === false) return null;
  return (
    <footer className="relative overflow-hidden bg-gradient-to-br from-slate-950 via-slate-900 to-blue-950 text-white">
      <div className="pointer-events-none absolute inset-0 opacity-40" aria-hidden="true">
        <div className="absolute -right-24 -top-32 h-96 w-96 rounded-full bg-blue-600/20 blur-3xl" />
        <div className="absolute -bottom-40 left-1/4 h-80 w-80 rounded-full bg-indigo-500/15 blur-3xl" />
      </div>

      <div className="relative mx-auto max-w-7xl px-5 pb-8 pt-12 sm:px-6 sm:pt-16 lg:px-8 lg:pt-20">
        <section aria-labelledby="footer-cta-title" className="mb-14 flex flex-col gap-7 rounded-3xl border border-white/10 bg-gradient-to-r from-blue-600/90 via-indigo-600/90 to-violet-600/90 p-7 shadow-2xl shadow-blue-950/20 sm:mb-16 sm:p-9 lg:flex-row lg:items-center lg:justify-between lg:p-10">
          <div className="max-w-2xl">
            <p className="text-xs font-bold uppercase tracking-[0.18em] text-blue-100">{t("Make every connection count")}</p>
            <h2 id="footer-cta-title" className="mt-2 text-2xl font-bold tracking-tight sm:text-3xl">{cmsText(cms.cta_heading)}</h2>
            <p className="mt-3 max-w-xl text-sm leading-6 text-blue-50 sm:text-base">{cmsText(cms.cta_description)}</p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row">
            {link(cms.cta_primary_url || "/register", <>{cmsText(cms.cta_primary_text || t("Get Started Free"))} <ArrowRight className="h-4 w-4" aria-hidden="true" /></>, "inline-flex min-h-12 items-center justify-center gap-2 rounded-xl bg-white px-5 py-3 text-sm font-bold text-blue-700 shadow-lg transition duration-200 hover:-translate-y-0.5 hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white")}
            {link(cms.cta_secondary_url || "/templates", cmsText(cms.cta_secondary_text || t("View Templates")), "inline-flex min-h-12 items-center justify-center rounded-xl border border-white/40 bg-white/10 px-5 py-3 text-sm font-semibold text-white transition duration-200 hover:bg-white/20 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white")}
          </div>
        </section>

        <div className="grid gap-10 sm:grid-cols-2 lg:grid-cols-[1.5fr_1fr_1fr_1fr] lg:gap-12">
          <div>
            <Link href="/" aria-label={t("DBIZCARD home")} className="inline-flex items-center gap-3 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400">
              <BrandLogo className="h-12 w-[190px] max-w-full rounded-sm bg-white p-1" />
            </Link>
            <span className="mt-4 inline-flex rounded-full border border-blue-300/20 bg-blue-400/10 px-3 py-1 text-xs font-semibold text-blue-200">{t("Digital Workspace")}</span>
            <p className="mt-4 max-w-sm text-sm font-medium text-slate-200">{cmsText(cms.company_description)}</p>
          </div>

          {groups.map((group: any, index: number) => (
            <nav key={group.title} aria-label={`${group.title} links`}>
            <h3 className="text-sm font-bold text-white">{group.title}</h3>
              <ul className="mt-4 space-y-3">
                {group.links.map((item: { label: string; href: string }) => (
                  <li key={`${item.href}-${index}`}>
                    {link(item.href, item.label, "text-sm text-slate-400 transition-colors duration-200 hover:text-white focus-visible:rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400")}
                  </li>
                ))}
              </ul>
            </nav>
          ))}
        </div>

        {Array.isArray(cms.social_links) && cms.social_links.filter((item: any) => item.enabled !== false && item.url).length > 0 && <div className="mt-8 flex flex-wrap gap-4">{cms.social_links.filter((item: any) => item.enabled !== false && item.url).map((item: any, index: number) => <a key={`${item.platform}-${index}`} href={item.url} target="_blank" rel="noopener noreferrer" className="text-sm font-semibold text-slate-300 hover:text-white">{item.platform || "Social"}</a>)}</div>}

        <div className="mt-12 flex flex-col gap-3 border-t border-white/10 pt-6 text-xs text-slate-500 sm:mt-14 sm:flex-row sm:items-center sm:justify-between">
          <p>{t(copyright)}</p>
          <Link href="/contact" className="w-fit text-slate-400 transition-colors duration-200 hover:text-white focus-visible:rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400">
            {t("Contact Sales")}
          </Link>
        </div>
      </div>
    </footer>
  );
}
