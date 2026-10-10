"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { adminApi } from "@/lib/admin-api";
import { featuresPageDefaults, readFeaturesPageContent } from "@/lib/features-page-content";

type Feature = { id: number; title: string; description: string; detailed_description: string; icon_key: string; illustration_url: string; sort_order: number; is_enabled: boolean; cta_label: string; cta_url: string };
type Form = Omit<Feature, "id">;
const empty = (): Form => ({ title: "", description: "", detailed_description: "", icon_key: "", illustration_url: "", sort_order: 0, is_enabled: true, cta_label: "", cta_url: "" });
const field = "mt-1 w-full rounded-xl border border-slate-300 px-3 py-2.5";

export default function FeaturesPage() {
  const [items, setItems] = useState<Feature[]>([]);
  const [page, setPage] = useState(featuresPageDefaults);
  const [pageAction, setPageAction] = useState<"save" | "reset" | "delete" | null>(null);
  const [confirmation, setConfirmation] = useState<"reset" | "delete" | null>(null);
  const pageBusy = useRef(false);
  const pageVersion = useRef(0);
  const [form, setForm] = useState<Form>(empty());
  const [editing, setEditing] = useState<number | null>(null);
  const [loading, setLoading] = useState(true); const [saving, setSaving] = useState(false); const [error, setError] = useState(""); const [notice, setNotice] = useState("");
  const load = useCallback(async () => {
    const version = pageVersion.current;
    setLoading(true); setError("");
    try {
      const [features, sectionResult] = await Promise.all([
        adminApi("/admin/website/features"),
        adminApi("/admin/website/sections/features"),
      ]);
      if (!Array.isArray(features?.items)) throw new Error("Feature list response is missing its items array.");
      setItems(features.items as Feature[]);
      if (!pageBusy.current && version === pageVersion.current) setPage(readFeaturesPageContent(sectionResult));
    }
    catch (e) { setError(e instanceof Error ? e.message : "Could not load features."); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  const pageDisabled = loading || saving || pageAction !== null;
  function setPageArabic(field:string,value:string){setPage(current=>({...current,translations:{...current.translations,ar:{...(current.translations.ar||{}),[field]:value}}}));}
  function setFeatureArabic(field:string,value:string){if(!editing)return;setPage(current=>{const ar=current.translations.ar||{};const featureItems=ar.feature_items||{};const item=featureItems[String(editing)]||{};return {...current,translations:{...current.translations,ar:{...ar,feature_items:{...featureItems,[String(editing)]:{...item,[field]:value}}}}};});}
  async function mutatePage(action: "save" | "reset" | "delete") {
    if (pageBusy.current || loading || saving) return;
    pageBusy.current = true;
    pageVersion.current += 1;
    setPageAction(action); setError(""); setNotice("");
    let completed = false;
    try {
      const result = await adminApi("/admin/website/sections/features" + (action === "reset" ? "/reset" : ""), {
        method: action === "save" ? "PUT" : action === "reset" ? "POST" : "DELETE",
        ...(action === "save" ? { body: JSON.stringify(page) } : {}),
      });
      completed = true;
      // A successful mutation response also prevents stale values if reloading fails.
      setPage(readFeaturesPageContent(action === "delete" ? {} : action === "reset" ? result.data : result.content));
      setConfirmation(null);
      const saved = await adminApi("/admin/website/sections/features");
      setPage(readFeaturesPageContent(saved));
      setNotice(action === "save" ? "Feature page heading and subtitle saved." : action === "reset" ? "Features page content reset to defaults." : "Custom Features page content deleted.");
    } catch {
      setError(completed ? "Features page content was updated, but could not be reloaded. Retry to load the saved content." : action === "reset" ? "Could not reset Features page content." : action === "delete" ? "Could not delete Features page content." : "Could not save Features page content.");
    } finally {
      pageVersion.current += 1;
      pageBusy.current = false;
      setPageAction(null);
    }
  }
  async function savePage(e: React.FormEvent) { e.preventDefault(); await mutatePage("save"); }
  async function saveItem(e: React.FormEvent) { e.preventDefault(); setSaving(true); setError(""); setNotice(""); try { await adminApi(editing ? `/admin/website/features/${editing}` : "/admin/website/features", { method: editing ? "PUT" : "POST", body: JSON.stringify(form) }); setNotice(editing ? "Feature updated." : "Feature added."); setEditing(null); setForm(empty()); await load(); } catch (e) { setError(e instanceof Error ? e.message : "Could not save feature"); } finally { setSaving(false); } }
  function edit(item: Feature) { const { id, ...rest } = item; setEditing(id); setForm(rest); }
  async function remove(item: Feature) { if (!window.confirm(`Delete “${item.title}”? This only removes the website feature description.`)) return; try { await adminApi(`/admin/website/features/${item.id}`, { method: "DELETE" }); setItems(current => current.filter(value => value.id !== item.id)); if (editing === item.id) { setEditing(null); setForm(empty()); } setNotice("Feature deleted."); await load(); } catch (e) { setError(e instanceof Error ? e.message : "Could not delete feature"); } }
  async function move(index: number, delta: number) { const next = [...items]; const target = index + delta; if (target < 0 || target >= next.length) return; [next[index], next[target]] = [next[target], next[index]]; setItems(next); try { await adminApi("/admin/website/features/order", { method: "PUT", body: JSON.stringify({ ids: next.map(x => x.id) }) }); setNotice("Feature order saved."); } catch (e) { setError(e instanceof Error ? e.message : "Could not save feature order"); await load(); } }
  function toggle(item: Feature) { void adminApi(`/admin/website/features/${item.id}`, { method: "PUT", body: JSON.stringify({ ...item, is_enabled: !item.is_enabled }) }).then(() => { setNotice(item.is_enabled ? "Feature disabled." : "Feature enabled."); void load(); }).catch(e => setError(e instanceof Error ? e.message : "Could not update feature")); }

  return <section className="max-w-6xl"><p className="text-sm text-blue-600">Website Management</p><h1 className="mt-1 text-3xl font-bold">Features</h1><p className="mt-2 text-sm text-slate-500">Edit the public page content and feature cards. Disabled items stay in the admin list.</p>
    {error && <p role="alert" className="mt-5 rounded-xl bg-red-50 p-3 text-sm text-red-700">{error}<button type="button" onClick={() => void load()} className="ml-3 underline">Retry</button></p>}{notice && <p role="status" className="mt-5 rounded-xl bg-emerald-50 p-3 text-sm text-emerald-800">{notice}</p>}
    <form onSubmit={savePage} className="mt-6 grid gap-4 rounded-2xl border bg-white p-5 sm:grid-cols-2">
      <h2 className="sm:col-span-2 text-lg font-bold">Page Content</h2>
      <label className="text-sm font-semibold">Page Heading<input name="page_heading" disabled={pageDisabled} value={page.page_heading} onChange={e => setPage({ ...page, page_heading: e.target.value })} className={field}/></label>
      <label className="text-sm font-semibold">Page Subtitle<input name="page_subtitle" disabled={pageDisabled} value={page.page_subtitle} onChange={e => setPage({ ...page, page_subtitle: e.target.value })} className={field}/></label>
      <label className="text-sm font-semibold">Page Heading — العربية<input lang="ar" dir="rtl" value={page.translations.ar?.page_heading||""} onChange={e=>setPageArabic("page_heading",e.target.value)} className={field}/></label>
      <label className="text-sm font-semibold">Page Subtitle — العربية<input lang="ar" dir="rtl" value={page.translations.ar?.page_subtitle||""} onChange={e=>setPageArabic("page_subtitle",e.target.value)} className={field}/></label>
      <label className="flex items-center gap-2 text-sm"><input name="enabled" disabled={pageDisabled} type="checkbox" checked={page.enabled} onChange={e => setPage({ ...page, enabled: e.target.checked })}/>Publish custom feature highlights</label>
      <div className="flex flex-wrap gap-3 sm:col-span-2">
        <button type="button" disabled={pageDisabled} onClick={() => setConfirmation("reset")} className="rounded-xl border border-amber-300 bg-amber-50 px-5 py-2.5 font-semibold text-amber-800 disabled:opacity-50">{pageAction === "reset" ? "Resetting…" : "Reset Page Content"}</button>
        <button type="button" disabled={pageDisabled} onClick={() => setConfirmation("delete")} className="rounded-xl bg-red-600 px-5 py-2.5 font-semibold text-white disabled:opacity-50">{pageAction === "delete" ? "Deleting…" : "Delete Page Content"}</button>
        <button disabled={pageDisabled} className="rounded-xl bg-blue-600 px-5 py-2.5 font-semibold text-white disabled:opacity-50">{pageAction === "save" ? "Saving…" : "Save Page Content"}</button>
      </div>
    </form>
    <form onSubmit={saveItem} className="mt-6 space-y-4 rounded-2xl border bg-white p-5"><h2 className="text-lg font-bold">{editing ? "Edit Feature" : "Add Feature"}</h2><div className="grid gap-4 sm:grid-cols-2"><label className="text-sm font-semibold">Title *<input required value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} className={field}/></label><label className="text-sm font-semibold">Title — العربية<input lang="ar" dir="rtl" disabled={!editing} value={editing?page.translations.ar?.feature_items?.[String(editing)]?.title||"":""} onChange={e=>setFeatureArabic("title",e.target.value)} className={field}/></label><label className="text-sm font-semibold">Icon key<input value={form.icon_key} onChange={e => setForm({ ...form, icon_key: e.target.value })} placeholder="credit-card" className={field}/></label><label className="text-sm font-semibold sm:col-span-2">Short description<textarea rows={2} value={form.description} onChange={e => setForm({ ...form, description: e.target.value })} className={field}/></label><label className="text-sm font-semibold sm:col-span-2">Short description — العربية<textarea lang="ar" dir="rtl" disabled={!editing} rows={2} value={editing?page.translations.ar?.feature_items?.[String(editing)]?.description||"":""} onChange={e=>setFeatureArabic("description",e.target.value)} className={field}/></label><label className="text-sm font-semibold sm:col-span-2">Detailed description<textarea rows={3} value={form.detailed_description} onChange={e => setForm({ ...form, detailed_description: e.target.value })} className={field}/></label><label className="text-sm font-semibold sm:col-span-2">Detailed description — العربية<textarea lang="ar" dir="rtl" disabled={!editing} rows={3} value={editing?page.translations.ar?.feature_items?.[String(editing)]?.detailed_description||"":""} onChange={e=>setFeatureArabic("detailed_description",e.target.value)} className={field}/></label><label className="text-sm font-semibold">Illustration URL<input type="url" value={form.illustration_url} onChange={e => setForm({ ...form, illustration_url: e.target.value })} className={field}/></label><label className="text-sm font-semibold">Sort order<input type="number" value={form.sort_order} onChange={e => setForm({ ...form, sort_order: Number(e.target.value) })} className={field}/></label><label className="text-sm font-semibold">CTA label<input value={form.cta_label} onChange={e => setForm({ ...form, cta_label: e.target.value })} className={field}/></label><label className="text-sm font-semibold">CTA label — العربية<input lang="ar" dir="rtl" disabled={!editing} value={editing?page.translations.ar?.feature_items?.[String(editing)]?.cta_label||"":""} onChange={e=>setFeatureArabic("cta_label",e.target.value)} className={field}/></label><label className="text-sm font-semibold">CTA URL<input value={form.cta_url} onChange={e => setForm({ ...form, cta_url: e.target.value })} placeholder="/register" className={field}/></label><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={form.is_enabled} onChange={e => setForm({ ...form, is_enabled: e.target.checked })}/>Enabled on public page</label>{!editing&&<p className="sm:col-span-2 text-sm text-slate-500">Add the English feature first. Arabic fields become available when editing the saved feature.</p>}</div><div className="flex gap-2"><button disabled={saving || loading} className="rounded-xl bg-blue-600 px-5 py-2.5 font-semibold text-white disabled:opacity-50">{saving ? "Saving…" : editing ? "Update Feature" : "Add Feature"}</button>{editing && <button type="button" onClick={() => { setEditing(null); setForm(empty()); }} className="rounded-xl border px-5 py-2.5">Cancel</button>}</div></form>
    {loading ? <p role="status" className="mt-6 rounded-2xl border bg-white p-8 text-center text-slate-500">Loading saved features…</p> : <div className="mt-6 space-y-3">{!error && items.length === 0 && <p className="rounded-2xl border bg-white p-8 text-center text-slate-500">No items found</p>}{items.map((item, i) => <article key={item.id} className="flex flex-wrap items-center gap-4 rounded-2xl border bg-white p-4"><div className="min-w-0 flex-1"><h3 className="font-bold">{item.title} <span className={`ml-2 rounded-full px-2 py-1 text-xs ${item.is_enabled ? "bg-emerald-50 text-emerald-700" : "bg-slate-100 text-slate-500"}`}>{item.is_enabled ? "Enabled" : "Disabled"}</span></h3><p className="mt-1 text-sm text-slate-500">{item.description}</p><p className="mt-1 text-xs text-slate-400">Order {item.sort_order} · {item.icon_key || "No icon"}</p></div><button type="button" onClick={() => void move(i, -1)} aria-label="Move feature up" disabled={i === 0} className="rounded-lg border px-3 py-2 disabled:opacity-40">↑</button><button type="button" onClick={() => void move(i, 1)} aria-label="Move feature down" disabled={i === items.length - 1} className="rounded-lg border px-3 py-2 disabled:opacity-40">↓</button><button type="button" onClick={() => toggle(item)} className="rounded-lg border px-3 py-2">{item.is_enabled ? "Disable" : "Enable"}</button><button type="button" onClick={() => edit(item)} className="rounded-lg border px-3 py-2">Edit</button><button type="button" onClick={() => void remove(item)} className="rounded-lg border px-3 py-2 text-red-700">Delete</button></article>)}</div>}
    {confirmation && <div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4" role="dialog" aria-modal="true" aria-labelledby="features-content-confirm-title" aria-describedby="features-content-confirm-description" onKeyDown={event => {
      if (event.key === "Escape" && !pageBusy.current) setConfirmation(null);
      if (event.key === "Tab") {
        const buttons = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>("button:not(:disabled)"));
        const first = buttons[0], last = buttons[buttons.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    }}><div className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl">
      <h2 id="features-content-confirm-title" className="text-xl font-bold">{confirmation === "reset" ? "Reset Features page content to defaults?" : "Delete custom Features page content?"}</h2>
      <p id="features-content-confirm-description" className="mt-3 text-sm text-slate-600">{confirmation === "reset" ? "This will replace the saved page heading/subtitle configuration with the existing defaults. Feature cards will not be changed." : "This will remove the saved page heading/subtitle configuration. Feature cards will not be deleted."}</p>
      {error && <p role="alert" className="mt-3 text-sm text-red-700">{error}</p>}
      <div className="mt-6 flex justify-end gap-3"><button type="button" autoFocus disabled={pageDisabled} onClick={() => setConfirmation(null)} className="rounded-xl border px-4 py-2 disabled:opacity-50">Cancel</button><button type="button" disabled={pageDisabled} onClick={() => mutatePage(confirmation)} className={"rounded-xl px-4 py-2 font-semibold text-white disabled:opacity-50 " + (confirmation === "delete" ? "bg-red-600" : "bg-amber-600")}>{pageAction ? "Working…" : confirmation === "reset" ? "Reset" : "Delete"}</button></div>
    </div></div>}
  </section>;
}
