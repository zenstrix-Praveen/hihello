"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Menu, X, LogIn, ArrowRight, ChevronDown } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { ReactNode } from "react";
import { ChatbotLauncher } from "@/components/public/Chatbot";
import { useWebsiteSection } from "@/components/public/useWebsiteSection";
import BrandLogo from "@/components/brand/BrandLogo";
import { platformMenuGroups } from "@/lib/platform-menu";
import { useCustomerLocale } from "@/lib/customer-locale";
import CustomerLanguageSwitcher from "@/components/locale/CustomerLanguageSwitcher";
import { navigationPath, publicNavigationHref } from "@/lib/public-navigation";

const defaultNavigation = [
  { label: "Home", href: "/" },
  { label: "Features", href: "/features" },
  { label: "Templates", href: "/templates" },
  { label: "Pricing", href: "/pricing" },
  { label: "Business", href: "/business" },
  { label: "Contact", href: "/contact" },
];

// Keep the component identity stable when the menu or scroll state changes.
function RenderLink({ href, children, className, onClick }: { href: string; children: ReactNode; className?: string; onClick?: () => void }) {
  const localHref = (href.startsWith("/") && !href.startsWith("//")) || href.startsWith("#");
  return localHref
    ? <Link href={href} className={className} onClick={onClick}>{children}</Link>
    : <a href={href} className={className} onClick={onClick} rel="noopener noreferrer">{children}</a>;
}

export default function PublicHeader(_props: { dbizcardBrand?: boolean }) {
  const { t } = useCustomerLocale();
  const config = useWebsiteSection<any>("header", { brand_name: "DBIZCARD", logo_url: "", navigation_items: null, login_button: { enabled: true, label: "Login", url: "/login" }, signup_button: { enabled: true, label: "Get Started", url: "/register" }, enabled: true }, { fallbackOnUnavailable: true });
  const seo = useWebsiteSection<any>("seo", { site_name: "DBIZCARD", title_suffix: " | DBIZCARD", default_title: "DBIZCARD | Digital Business Cards", meta_description: "Create, share and manage your DBIZCARD digital business cards.", meta_keywords: [], favicon_url: "/brand/dbizcard-icon.png", default_og_image_url: "", robots_enabled: true });
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [platformOpen, setPlatformOpen] = useState(false);
  const [mobilePlatformOpen, setMobilePlatformOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const platformTriggerRef = useRef<HTMLButtonElement>(null);
  const platformPanelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleScroll = () => {
      setScrolled(window.scrollY > 10);
    };

    handleScroll();

    window.addEventListener("scroll", handleScroll);

    return () => {
      window.removeEventListener("scroll", handleScroll);
    };
  }, []);

  useEffect(() => {
    setMobileOpen(false);
    setPlatformOpen(false);
    setMobilePlatformOpen(false);
  }, [pathname]);

  useEffect(() => {
    const closeNavigation = () => { setMobileOpen(false); setMobilePlatformOpen(false); };
    const closeChat = () => window.dispatchEvent(new Event("dbizcard:close-chat"));
    window.addEventListener("dbizcard:close-navigation", closeNavigation);
    window.addEventListener("dbizcard:close-chat-request", closeChat);
    return () => {
      window.removeEventListener("dbizcard:close-navigation", closeNavigation);
      window.removeEventListener("dbizcard:close-chat-request", closeChat);
    };
  }, []);

  useEffect(() => {
    if (!platformOpen) return;
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!platformTriggerRef.current?.contains(target) && !platformPanelRef.current?.contains(target)) setPlatformOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") { setPlatformOpen(false); platformTriggerRef.current?.focus(); }
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => { document.removeEventListener("pointerdown", onPointerDown); document.removeEventListener("keydown", onKeyDown); };
  }, [platformOpen]);

  useEffect(() => {
    if (seo.enabled === false) return;
    const title = document.querySelector('meta[name="page-title"]')?.getAttribute("content");
    const brandSafe = (value: string) => value.replace(/hihello|dbizcard/gi, "DBIZCARD");
    document.title = brandSafe(title || (seo.default_title ? `${seo.default_title}${seo.title_suffix || ""}` : `${seo.site_name || "DBIZCARD"} | Digital Business Cards`));
    let description = document.querySelector('meta[name="description"]');
    if (!description) { description = document.createElement("meta"); description.setAttribute("name", "description"); document.head.appendChild(description); }
    description.setAttribute("content", brandSafe(seo.meta_description || "Create, share and manage your DBIZCARD digital business cards."));
    let ogTitle = document.querySelector<HTMLMetaElement>('meta[property="og:title"]'); if (!ogTitle) { ogTitle = document.createElement("meta"); ogTitle.setAttribute("property", "og:title"); document.head.appendChild(ogTitle); } ogTitle.content = brandSafe(seo.default_title || seo.site_name || "DBIZCARD");
    let ogDescription = document.querySelector<HTMLMetaElement>('meta[property="og:description"]'); if (!ogDescription) { ogDescription = document.createElement("meta"); ogDescription.setAttribute("property", "og:description"); document.head.appendChild(ogDescription); } ogDescription.content = brandSafe(seo.meta_description || "Create, share and manage your DBIZCARD digital business cards.");
    let ogSite = document.querySelector<HTMLMetaElement>('meta[property="og:site_name"]'); if (!ogSite) { ogSite = document.createElement("meta"); ogSite.setAttribute("property", "og:site_name"); document.head.appendChild(ogSite); } ogSite.content = brandSafe(seo.site_name || "DBIZCARD");
    let robots = document.querySelector('meta[name="robots"]');
    if (!robots) { robots = document.createElement("meta"); robots.setAttribute("name", "robots"); document.head.appendChild(robots); }
    robots.setAttribute("content", seo.robots_enabled === false ? "noindex, nofollow" : "index, follow");
    if (seo.favicon_url) { let icon = document.querySelector<HTMLLinkElement>('link[rel="icon"]'); if (!icon) { icon = document.createElement("link"); icon.rel = "icon"; document.head.appendChild(icon); } icon.href = seo.favicon_url; }
    if (seo.default_og_image_url) { let image = document.querySelector<HTMLMetaElement>('meta[property="og:image"]'); if (!image) { image = document.createElement("meta"); image.setAttribute("property", "og:image"); document.head.appendChild(image); } image.content = seo.default_og_image_url; }
    if (Array.isArray(seo.meta_keywords)) { let keywords = document.querySelector<HTMLMetaElement>('meta[name="keywords"]'); if (!keywords) { keywords = document.createElement("meta"); keywords.name = "keywords"; document.head.appendChild(keywords); } keywords.content = seo.meta_keywords.join(", "); }
    if (seo.public_base_url) { let canonical = document.querySelector<HTMLLinkElement>('link[rel="canonical"]'); if (!canonical) { canonical = document.createElement("link"); canonical.rel = "canonical"; document.head.appendChild(canonical); } canonical.href = `${String(seo.public_base_url).replace(/\/$/, "")}${window.location.pathname}`; }
  }, [seo, pathname]);

  function isActive(href: string) {
    if (href === "/") {
      return pathname === "/";
    }

    return pathname === href || pathname.startsWith(`${href}/`);
  }
  const normalizeHref = (href: string) => publicNavigationHref(href, seo.public_base_url, typeof window === "undefined" ? undefined : window.location.origin);
  const navigation: { label: string; href: string }[] = Array.isArray(config.navigation_items)
    ? config.navigation_items.filter((item: any) => {
        if (item.enabled === false || !item.label || !item.url) return false;
        const label = String(item.label).trim().toLowerCase();
        const href = navigationPath(normalizeHref(String(item.url)));
        return !["about us", "faq", "frequently asked questions"].includes(label) && !["/about", "/faq"].includes(href);
      }).map((item: any) => ({ label: String(item.label), href: normalizeHref(String(item.url)) }))
    : defaultNavigation;
  const platformConfig = config.platform_menu || { label: "Platform", enabled: true };
  const platformLabel = String(platformConfig.label || "Platform");
  const showPlatform = platformConfig.enabled !== false;
  // Platform is a menu, not an App Router page. CMS labels may be translated.
  const regularNavigation = navigation.filter(item => item.label.trim().toLowerCase() !== "platform" && navigationPath(item.href) !== "/platform" && item.href !== "#platform-menu");
  const displayNavigation: { label: string; href: string; isPlatform?: boolean }[] = [];
  for (const item of regularNavigation) {
    displayNavigation.push(item);
    if (showPlatform && navigationPath(item.href) === "/features") displayNavigation.push({ label: platformLabel, href: "#platform-menu", isPlatform: true });
  }
  if (showPlatform && !displayNavigation.some(item => item.isPlatform)) displayNavigation.splice(1, 0, { label: platformLabel, href: "#platform-menu", isPlatform: true });
  const action = (value: any, fallback: { label: string; url: string }) => ({ ...fallback, ...(value || {}), url: normalizeHref(String(value?.url || fallback.url)) });
  const login = action(config.login_button, { label: "Login", url: "/login" });
  const signup = action(config.signup_button, { label: "Get Started", url: "/register" });

  return (
    <>
    <header
      data-public-header
      className={`sticky top-0 z-50 border-b transition-all duration-300 ${
        scrolled
          ? "border-slate-200 bg-white/95 shadow-sm backdrop-blur-xl"
          : "border-slate-100 bg-white"
      }`}
    >
      <div className="mx-auto grid min-h-[71px] w-full max-w-7xl grid-cols-[190px_minmax(0,1fr)_auto] items-center gap-3 px-4 sm:px-6 lg:gap-4 lg:px-8">

        {/* LOGO */}
        <Link
          href="/"
          className="flex shrink-0 items-center"
          aria-label={t("DBIZCARD home")}
        >
          <BrandLogo priority className="h-12 w-[190px] max-w-full" />
        </Link>

        {/* DESKTOP NAVIGATION */}
        <nav className="hidden min-w-0 items-center justify-center gap-6 min-[1280px]:flex">
          {displayNavigation.map((item) => {
            if (item.isPlatform) return <button key="platform-menu-trigger" ref={platformTriggerRef} type="button" onClick={() => setPlatformOpen(value => !value)} onMouseEnter={() => setPlatformOpen(true)} aria-expanded={platformOpen} aria-haspopup="true" aria-controls="platform-mega-menu" className={`flex items-center gap-1 whitespace-nowrap rounded-lg px-0 py-2 text-sm font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 ${platformOpen ? "bg-blue-50 text-blue-700" : "text-slate-700 hover:bg-slate-50 hover:text-blue-600"}`}>
              {t(item.label)}<ChevronDown aria-hidden="true" className={`h-4 w-4 transition-transform ${platformOpen ? "rotate-180" : ""}`} />
            </button>;
            const active = isActive(item.href);

            return (
              <RenderLink
                key={item.href}
                href={item.href}
                  className={`group relative whitespace-nowrap rounded-lg px-0 py-2 text-sm font-medium transition ${
                  active
                    ? "text-blue-600"
                    : "text-slate-700 hover:bg-slate-50 hover:text-blue-600"
                }`}
              >
                {t(item.label)}

                <span
                  className={`absolute inset-x-2 bottom-1 h-0.5 origin-start rounded-full bg-blue-600 transition-transform duration-300 ${
                    active
                      ? "scale-x-100"
                      : "scale-x-0 group-hover:scale-x-100"
                  }`}
                />
              </RenderLink>
            );
          })}
        </nav>

        {/* DESKTOP ACTIONS */}
        <div className="hidden shrink-0 items-center gap-2 min-[1280px]:flex">
          {login.enabled !== false && <RenderLink
            href={login.url || "/login"}
            className={`flex h-10 items-center gap-1.5 whitespace-nowrap rounded-xl px-2 py-2 text-sm font-medium transition ${
              isActive("/login")
                ? "bg-blue-50 text-blue-700"
                : "text-slate-700 hover:bg-slate-100 hover:text-blue-600"
            }`}
          >
            <LogIn className="h-4 w-4" />
            {t(login.label || "Login")}
          </RenderLink>}

          {signup.enabled !== false && <RenderLink
            href={signup.url || "/register"}
            className="group flex h-10 items-center gap-1.5 whitespace-nowrap rounded-xl bg-blue-600 px-4 text-sm font-semibold text-white shadow-sm transition hover:-translate-y-0.5 hover:bg-blue-700 hover:shadow-lg"
          >
            {t(signup.label || "Get Started")}

            <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-1 rtl:group-hover:-translate-x-1" />
          </RenderLink>}
          <Link href="/admin/login" className="h-10 whitespace-nowrap px-1 text-sm font-medium text-slate-500 hover:text-blue-600 inline-flex items-center">{t(config.admin_link_label || "Administrator")}</Link>
          <CustomerLanguageSwitcher />
        </div>

        {/* MOBILE BUTTON */}
        <button
          type="button"
          onClick={() => {
            window.dispatchEvent(new Event("dbizcard:close-chat-request"));
            setMobileOpen((value) => !value);
          }}
          className="relative z-[70] flex h-11 w-11 shrink-0 touch-manipulation select-none items-center justify-center rounded-xl border border-slate-200 text-slate-700 transition hover:bg-slate-100 min-[1280px]:hidden"
          aria-label={t(mobileOpen ? "Close navigation" : "Open navigation")}
          aria-controls="public-mobile-navigation"
          aria-expanded={mobileOpen}
        >
          {mobileOpen ? (
            <X className="h-6 w-6" />
          ) : (
            <Menu className="h-6 w-6" />
          )}
        </button>
      </div>

      {platformOpen && <div className="absolute inset-x-0 top-full z-[60] hidden px-4 min-[1280px]:block"><div ref={platformPanelRef} id="platform-mega-menu" role="region" aria-label={`${platformLabel} capabilities`} onMouseEnter={() => setPlatformOpen(true)} className="mx-auto max-w-[1200px] rounded-b-3xl border border-slate-200 bg-white p-6 shadow-2xl shadow-slate-900/15 lg:p-8">
        <div className="grid gap-7 sm:grid-cols-2 xl:grid-cols-4">{platformMenuGroups.map(group => <section key={group.heading} aria-label={group.heading}>
          <h2 className="mb-3 text-xs font-extrabold uppercase tracking-[.16em] text-slate-500">{t(group.heading)}</h2>
          <ul className="space-y-1">{group.items.map(menuItem => { const Icon = menuItem.icon; return <li key={menuItem.title}><Link href={menuItem.href} onClick={() => setPlatformOpen(false)} className="group flex gap-3 rounded-xl p-2.5 transition hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600">
            <span className="mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-blue-50 text-blue-700 transition group-hover:bg-white"><Icon aria-hidden="true" className="h-[18px] w-[18px]"/></span>
            <span className="min-w-0"><span className="block text-sm font-semibold text-slate-900 group-hover:text-blue-700">{t(menuItem.title)}</span><span className="mt-0.5 block text-xs leading-5 text-slate-500">{t(menuItem.description)}</span></span>
          </Link></li>; })}</ul>
        </section>)}</div>
      </div></div>}

      {/* MOBILE NAVIGATION */}
      <div
        id="public-mobile-navigation"
        aria-hidden={!mobileOpen}
        inert={!mobileOpen}
        className={`relative z-[55] overflow-y-auto border-t border-slate-100 bg-white transition-all duration-300 min-[1280px]:hidden ${
          mobileOpen
            ? "max-h-[calc(100dvh-72px)] opacity-100"
            : "pointer-events-none max-h-0 border-t-transparent opacity-0"
        }`}
      >
        <nav className="mx-auto max-w-7xl space-y-1 px-5 py-5">
          {displayNavigation.map((item) => {
            if (item.isPlatform) return <div key="platform-mobile-menu" className="rounded-xl border border-slate-200 bg-white">
              <button type="button" onClick={() => setMobilePlatformOpen(value => !value)} aria-expanded={mobilePlatformOpen} aria-controls="platform-mobile-groups" className="flex min-h-12 w-full items-center justify-between rounded-xl px-4 text-start font-semibold text-slate-800 hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600">
                {t(item.label)}<ChevronDown aria-hidden="true" className={`h-4 w-4 transition-transform ${mobilePlatformOpen ? "rotate-180" : ""}`} />
              </button>
              {mobilePlatformOpen && <div id="platform-mobile-groups" className="space-y-4 border-t border-slate-100 px-4 py-4">{platformMenuGroups.map(group => <section key={group.heading} aria-label={t(group.heading)}><h3 className="mb-2 text-[11px] font-bold uppercase tracking-[.16em] text-slate-500">{t(group.heading)}</h3><div className="space-y-1">{group.items.map(menuItem => <Link key={menuItem.title} href={menuItem.href} onClick={() => { setPlatformOpen(false); setMobilePlatformOpen(false); setMobileOpen(false); }} className="flex min-h-11 items-center rounded-lg px-3 text-sm font-semibold text-slate-700 hover:bg-blue-50 hover:text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600">{t(menuItem.title)}</Link>)}</div></section>)}</div>}
            </div>;
            const active = isActive(item.href);

            return (
              <RenderLink
                key={item.href}
                href={item.href}
                onClick={() => { setMobileOpen(false); setMobilePlatformOpen(false); }}
                className={`flex min-h-12 items-center rounded-xl px-4 font-semibold transition ${
                  active
                    ? "bg-blue-50 text-blue-700"
                    : "text-slate-700 hover:bg-slate-50"
                }`}
              >
                {t(item.label)}
              </RenderLink>
            );
          })}

          <div className="my-4 border-t border-slate-100" />

          {login.enabled !== false && <RenderLink
            href={login.url || "/login"}
            onClick={() => setMobileOpen(false)}
            className="flex min-h-12 items-center gap-2 rounded-xl px-4 font-semibold text-slate-700 hover:bg-slate-50"
          >
            <LogIn className="h-5 w-5" />
            {t(login.label || "Login")}
          </RenderLink>}

          {signup.enabled !== false && <RenderLink
            href={signup.url || "/register"}
            onClick={() => setMobileOpen(false)}
            className="mt-3 flex min-h-12 items-center justify-center gap-2 rounded-xl bg-blue-600 px-5 font-bold text-white"
          >
            {t(signup.label || "Get Started")}
            <ArrowRight className="h-4 w-4" />
          </RenderLink>}
          <Link href="/admin/login" onClick={() => setMobileOpen(false)} className="flex min-h-12 items-center rounded-xl px-4 text-sm font-semibold text-slate-500 hover:bg-slate-50">{t(config.admin_link_label || "Login as Administrator")}</Link>
          <div className="px-4 py-2"><CustomerLanguageSwitcher /></div>
        </nav>
      </div>
    </header>
    <ChatbotLauncher audience="public" />
    </>
  );
}
