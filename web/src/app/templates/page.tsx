"use client";
import { getApiBaseUrl } from "@/lib/api-config";
import { useEffect, useRef, useState } from "react";
import PublicHeader from "@/components/public/PublicHeader";
import PublicFooter from "@/components/public/PublicFooter";
import TemplatePreview, { designFor } from "@/components/templates/TemplatePreview";
import { getUserToken } from "@/lib/user-api";
import type { DesignSettings } from "@/components/dashboard/data";
import { useWebsiteSection } from "@/components/public/useWebsiteSection";
import { useCustomerLocale } from "@/lib/customer-locale";

type Template = { id: number; name: string; description?: string; preview_image_url?: string; status?: string; slug?: string; category?: string; design_settings?: Partial<DesignSettings>; _locale_fallback?: boolean };
const API = getApiBaseUrl();
const categories = ["All", "Professional", "Modern", "Minimal", "Creative", "Corporate", "Personal", "Luxury", "Dark", "Colorful", "Gradient"];

export default function TemplatesPage() {
  const {t,language,error:localizeError}=useCustomerLocale();
  const content = useWebsiteSection<any>("templates", { page_title: "Choose a design that represents you.", page_subtitle: "Start with a professionally designed digital business card and customize the colors, typography and content to match your brand.", hero_text: "Digital Card Templates", cta_text: "Explore Templates", cta_url: "/templates" }, { fallbackOnUnavailable: true });
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [preview, setPreview] = useState<Template | null>(null);
  const [category, setCategory] = useState("All");
  const closeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    let active = true;
    fetch(`${API}/website/templates?lang=${language}`, { cache: "no-store" }).then(async response => {
      if (!response.ok) throw new Error("Could not load templates.");
      return response.json();
    }).then(result => {
      if (active) setTemplates(Array.isArray(result) ? result : Array.isArray(result?.data) ? result.data : []);
    }).catch(err => { if (active) setError(localizeError(err)); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [localizeError,language]);

  useEffect(() => {
    if (!preview) return;
    closeRef.current?.focus();
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape") setPreview(null); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [preview]);

  const visibleTemplates = templates.filter(item => category === "All" || t(item.category || "").toLowerCase() === t(category).toLowerCase());
  useEffect(()=>{const untranslated=templates.some(item=>item._locale_fallback&&[item.name,item.description||""].some(value=>value&&t(value)===value));window.dispatchEvent(new CustomEvent("website-locale-fallback",{detail:{section:"template-records",fallback:language==="ar"&&untranslated}}));},[templates,t,language]);
  function useTemplate(id: number) { window.location.assign(getUserToken() ? `/dashboard/cards/new?template=${id}` : `/register?template=${id}`); }

  if (content.enabled === false) return <><PublicHeader /><PublicFooter /></>;
  return <>
    <PublicHeader />
    <main className="min-h-[700px] bg-[#F8FAFC]">
      <section className="relative overflow-hidden border-b border-blue-100 bg-gradient-to-br from-white via-blue-50 to-indigo-100/80 py-12 sm:py-14">
        <div aria-hidden="true" className="absolute -right-20 -top-32 h-80 w-80 rounded-full bg-cyan-200/30 blur-3xl"/>
        <div className="relative mx-auto max-w-7xl px-5 lg:px-8">
          <p className="text-xs font-bold uppercase tracking-[.2em] text-blue-700">{content.hero_text}</p>
          <h1 className="mt-3 max-w-3xl text-4xl font-black tracking-tight text-slate-950 sm:text-5xl">{content.page_title}</h1>
          <p className="mt-4 max-w-2xl text-base leading-7 text-slate-600 sm:text-lg">{content.page_subtitle}</p>
          {content.cta_text && <a href={content.cta_url || "/templates"} className="mt-5 inline-flex rounded-xl bg-blue-600 px-5 py-3 font-bold text-white hover:bg-blue-700">{content.cta_text}</a>}
        </div>
      </section>

      <section className="mx-auto max-w-[1600px] px-4 py-8 sm:px-6 sm:py-11 lg:px-8">
        {templates.length > 0 && <nav aria-label={t("Filter templates")} className="sticky top-2 z-20 mb-7 -mx-1 flex gap-2 overflow-x-auto rounded-2xl border border-slate-200/80 bg-white/90 p-2 shadow-sm backdrop-blur sm:mx-0 sm:mb-8 sm:w-fit">
          {categories.map(item => <button key={item} type="button" onClick={() => setCategory(item)} aria-pressed={category === item} className={`shrink-0 rounded-full border px-4 py-2 text-sm font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 ${category === item ? "border-transparent bg-gradient-to-r from-blue-600 to-indigo-600 text-white shadow-sm" : "border-transparent bg-white text-slate-600 hover:border-blue-100 hover:bg-blue-50"}`}>{t(item)}</button>)}
        </nav>}

        {loading && <div className="grid gap-6 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">{[0, 1, 2, 3].map(index => <div key={index} aria-hidden="true" className="overflow-hidden rounded-3xl border border-slate-200 bg-white"><div className="h-[370px] animate-pulse bg-gradient-to-br from-slate-100 via-blue-50 to-indigo-100"/><div className="space-y-3 p-5"><div className="h-3 w-24 animate-pulse rounded bg-slate-100"/><div className="h-6 w-2/3 animate-pulse rounded bg-slate-100"/><div className="h-4 w-full animate-pulse rounded bg-slate-100"/><div className="mt-4 h-10 animate-pulse rounded-xl bg-slate-100"/></div></div>)}</div>}
        {!loading && error && <div role="alert" className="rounded-2xl border border-amber-200 bg-amber-50 p-6 text-amber-900"><p className="font-semibold">{t("Templates could not be loaded.")}</p><p className="mt-2 text-sm">{t("Please try again in a moment.")}</p></div>}
        {!loading && !error && templates.length === 0 && <div className="rounded-3xl border border-slate-200 bg-white p-10 text-center"><h2 className="text-xl font-bold text-slate-900">{t("No templates available")}</h2><p className="mt-2 text-slate-500">{t("Published templates will appear here.")}</p></div>}
        {!loading && !error && templates.length > 0 && visibleTemplates.length === 0 && <div className="rounded-3xl border border-slate-200 bg-white p-10 text-center"><h2 className="text-xl font-bold text-slate-900">{t("No templates in this category yet.")}</h2><button type="button" onClick={() => setCategory("All")} className="mt-4 rounded-xl bg-blue-600 px-5 py-3 text-sm font-semibold text-white hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">{t("View All Templates")}</button></div>}

        {!loading && !error && visibleTemplates.length > 0 && <div className="grid items-start gap-6 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          {visibleTemplates.map(template => {
            const design = designFor(template);
            const palette = [design.primary_color, design.secondary_color, design.accent_color];
            return <article key={template.id} className="group overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-sm transition duration-300 hover:-translate-y-1 hover:border-blue-200 hover:shadow-xl">
              <TemplatePreview template={template}/>
              <div className="p-5 sm:p-6">
                <p className="text-[10px] font-bold uppercase tracking-[.16em] text-blue-700">{t(template.category || "Dbizcard design")}</p>
                <h2 className="mt-1 text-xl font-bold tracking-tight text-slate-900">{t(template.name)}</h2>
                <p className="mt-2 line-clamp-2 min-h-10 text-sm leading-5 text-slate-500">{t(template.description || "A Dbizcard design ready to make your own.")}</p>
                <div className="mt-4 flex flex-wrap items-center gap-2"><span className="mr-1 text-xs font-semibold text-slate-500">{t("Color palette")}</span>{palette.map((color, index) => <span key={`${template.id}-${color}-${index}`} title={color} aria-label={t("Palette color {color}",{color})} className="h-5 w-5 rounded-full border border-black/10 ring-2 ring-white" style={{ backgroundColor: color }}/>)}</div>
                <div className="mt-5 grid grid-cols-2 gap-2.5">
                  <button type="button" onClick={() => setPreview(template)} className="rounded-xl border border-slate-300 px-3 py-3 text-sm font-semibold text-slate-700 transition hover:border-blue-300 hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">{t("Preview")}</button>
                  <button type="button" onClick={() => useTemplate(template.id)} className="rounded-xl bg-blue-600 px-3 py-3 text-sm font-semibold text-white transition hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2">{t("Use Template")}</button>
                </div>
              </div>
            </article>;
          })}
        </div>}
      </section>
    </main>

    {preview && <div role="presentation" className="fixed inset-0 z-[60] grid place-items-center bg-slate-950/65 p-3 backdrop-blur-sm sm:p-5" onMouseDown={event => { if (event.target === event.currentTarget) setPreview(null); }}>
      <section role="dialog" aria-modal="true" aria-labelledby="template-preview-title" className="max-h-[94dvh] w-full max-w-6xl overflow-auto rounded-3xl bg-white shadow-2xl">
        <div className="sticky top-0 z-10 flex items-center justify-between gap-4 border-b border-slate-100 bg-white/95 px-5 py-4 backdrop-blur sm:px-7">
          <div className="min-w-0"><p className="text-[10px] font-bold uppercase tracking-[.16em] text-blue-700">{t(preview.category || "Digital card template")}</p><h2 id="template-preview-title" className="mt-1 truncate text-xl font-bold text-slate-900">{t(preview.name)}</h2></div>
          <button ref={closeRef} type="button" aria-label={t("Close template preview")} onClick={() => setPreview(null)} className="shrink-0 rounded-xl border border-slate-200 px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">{t("Close")}</button>
        </div>
        <div className="grid gap-5 bg-slate-50 p-3 sm:p-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(280px,.8fr)]">
          <TemplatePreview template={preview} large/>
          <div className="flex items-center rounded-2xl bg-white p-6 sm:p-8"><div>
            <p className="text-xs font-bold uppercase tracking-[.16em] text-indigo-600">{t(preview.category || "Dbizcard template")}</p>
            <h3 className="mt-3 text-2xl font-bold tracking-tight text-slate-900">{t("A closer look at {name}",{name:t(preview.name)})}</h3>
            <p className="mt-3 text-sm leading-6 text-slate-600">{t(preview.description || "A thoughtfully designed starting point for your digital identity.")} {t("Your profile and contact information stay yours as you personalize the design in the card editor.")}</p>
            <div className="mt-6"><p className="text-xs font-bold uppercase tracking-wider text-slate-500">{t("Template palette")}</p><div className="mt-3 flex gap-2">{[designFor(preview).primary_color, designFor(preview).secondary_color, designFor(preview).accent_color].map((color, index) => <span key={`${preview.id}-${color}-${index}`} className="h-8 w-8 rounded-full border border-black/10" style={{ backgroundColor: color }}/>)}</div></div>
            <ul className="mt-6 space-y-2 text-sm text-slate-600"><li>✓ {t("Contact actions and profile details")}</li><li>✓ {t("Mobile friendly public card")}</li><li>✓ {t("Customize colors, typography and layout")}</li></ul>
            <button type="button" onClick={() => useTemplate(preview.id)} className="mt-7 w-full rounded-xl bg-blue-600 px-5 py-3.5 font-semibold text-white hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2">{t("Use Template")}</button>
          </div></div>
        </div>
      </section>
    </div>}
    <PublicFooter />
  </>;
}
