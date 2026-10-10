"use client";
import { useCustomerLocale } from "@/lib/customer-locale";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import TemplatePreview, { designFor } from "@/components/templates/TemplatePreview";
import { Template, useUserData } from "@/components/dashboard/data";
import { LoadState } from "@/components/dashboard/Widgets";

const categories = ["All", "Professional", "Modern", "Minimal", "Creative", "Corporate", "Personal", "Luxury", "Dark", "Colorful", "Gradient"];

export default function TemplatesPage() {
  const { t } = useCustomerLocale();
  const state = useUserData<Template[]>("/website/templates");
  const [category, setCategory] = useState("All");
  const [preview, setPreview] = useState<Template | null>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!preview) return;
    closeRef.current?.focus();
    const close = (event: KeyboardEvent) => { if (event.key === "Escape") setPreview(null); };
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [preview]);
  const visible = (state.data || []).filter(t => category === "All" || (t.category || "").toLowerCase() === category.toLowerCase());

  return <div className="space-y-6">
    <div><p className="text-xs font-bold uppercase tracking-[.18em] text-blue-700">{t("Dbizcard Designs")}</p><h1 className="mt-2 text-3xl font-bold">{t("Template Library")}</h1><p className="mt-2 text-slate-500">{t("Choose an original design, then customize it for your identity.")}</p></div>
    <LoadState {...state} retry={state.reload}/>
    {state.data && <>
      <nav aria-label={t("Filter templates")} className="flex gap-2 overflow-x-auto pb-1">{categories.map(item => <button key={item} type="button" onClick={() => setCategory(item)} aria-pressed={category === item} className={`shrink-0 rounded-full border px-4 py-2 text-sm font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 ${category === item ? "border-blue-600 bg-gradient-to-r from-blue-600 to-indigo-600 text-white" : "border-slate-200 bg-white text-slate-600 hover:border-blue-200"}`}>{t(item)}</button>)}</nav>
      {visible.length > 0 ? <div className="grid gap-6 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">{visible.map(template => {
        const design = designFor(template);
        return <article key={template.id} className="group overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-sm transition duration-300 hover:-translate-y-1 hover:border-blue-200 hover:shadow-xl">
          <TemplatePreview template={template}/>
          <div className="p-5"><p className="text-[10px] font-bold uppercase tracking-[.16em] text-blue-700">{t(template.category || "Dbizcard design")}</p><h2 className="mt-1 text-lg font-bold text-slate-900">{t(template.name)}</h2><p className="mt-2 line-clamp-2 min-h-10 text-sm text-slate-500">{t(template.description || "A Dbizcard design ready to make your own.")}</p>
            <div className="mt-3 flex gap-2">{[design.primary_color, design.secondary_color, design.accent_color].map((color, index) => <span key={`${template.id}-${color}-${index}`} title={color} className="h-5 w-5 rounded-full border border-black/10" style={{ backgroundColor: color }}/>)}</div>
            <div className="mt-4 grid grid-cols-2 gap-2"><button type="button" onClick={() => setPreview(template)} className="rounded-xl border px-3 py-2.5 text-sm font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">{t("Preview")}</button><Link href={`/dashboard/cards/new?template=${template.id}`} className="rounded-xl bg-blue-600 px-3 py-2.5 text-center text-sm font-semibold text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">{t("Use Template")}</Link></div>
          </div>
        </article>;
      })}</div> : <div className="rounded-2xl border bg-white p-8 text-center text-slate-500">{t("No templates in this category yet.")}<button type="button" onClick={() => setCategory("All")} className="ms-2 font-semibold text-blue-700 underline">{t("View all templates")}</button></div>}
    </>}
    {preview && <div role="presentation" className="fixed inset-0 z-50 grid place-items-center bg-slate-950/60 p-3 backdrop-blur-sm sm:p-5" onMouseDown={event => { if (event.target === event.currentTarget) setPreview(null); }}>
      <section role="dialog" aria-modal="true" aria-labelledby="dashboard-template-preview-title" className="max-h-[94dvh] w-full max-w-6xl overflow-auto rounded-3xl bg-white shadow-2xl">
        <div className="sticky top-0 z-10 flex items-center justify-between gap-4 border-b bg-white/95 p-5 backdrop-blur sm:p-7"><div><h2 id="dashboard-template-preview-title" className="text-xl font-bold">{t(preview.name)}</h2><p className="mt-1 text-sm text-slate-500">{t(preview.description || "")}</p></div><button ref={closeRef} type="button" aria-label={t("Close template preview")} onClick={() => setPreview(null)} className="rounded-lg px-3 py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">{t("Close")}</button></div>
        <div className="grid gap-5 bg-slate-50 p-3 sm:p-6 lg:grid-cols-[minmax(0,1.2fr)_minmax(280px,.8fr)]"><TemplatePreview template={preview} large/><div className="flex items-center rounded-2xl bg-white p-6"><div><p className="text-sm leading-6 text-slate-600">{t("This design is a starting point. Your card details stay yours, and you can adjust its colors, fonts and layout in the editor.")}</p><Link href={`/dashboard/cards/new?template=${preview.id}`} className="mt-5 inline-flex w-full justify-center rounded-xl bg-blue-600 px-5 py-3 font-semibold text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">{t("Use Template")}</Link></div></div></div>
      </section>
    </div>}
  </div>;
}
