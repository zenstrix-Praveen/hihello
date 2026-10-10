"use client";

import { useEffect, useState } from "react";
import { useAdminLocale } from "@/lib/admin-locale";
import { adminApi } from "@/lib/admin-api";

export default function WebsiteHomeAdminPage() {
  const { t } = useAdminLocale();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [resetting, setResetting] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [form, setForm] = useState({
    hero_title: "",
    hero_subtitle: "",
    primary_button_text: "",
    primary_button_url: "",
    secondary_button_text: "",
    secondary_button_url: "",
    hero_image_url: "",
    translations: { ar: {} as Record<string,string> },
  });
  const [contentLanguage, setContentLanguage] = useState<"en"|"ar">("en");

  useEffect(() => {
    adminApi("/admin/website/home").then((saved) => setForm(current => ({ ...current, ...saved })))
      .catch(e => setError(e instanceof Error ? e.message : "Could not load home settings"))
      .finally(() => setLoading(false));
  }, []);

  function updateField(
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) {
    const { name, value } = e.target;

    if (name.startsWith("ar_")) {
      const key=name.slice(3);
      setForm(current=>({...current,translations:{...current.translations,ar:{...current.translations.ar,[key]:value}}}));
    } else setForm((current) => ({...current,[name]: value}));
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (loading || saving || resetting) return;
    setSaving(true); setError(""); setNotice("");
    try {
      const saved = await adminApi("/admin/website/home", { method: "PUT", body: JSON.stringify(form) });
      setForm(current => ({ ...current, ...saved }));
      setNotice("Home page settings saved successfully.");
    } catch (err) { setError(err instanceof Error ? err.message : "Could not save home page settings"); }
    finally { setSaving(false); }
  }

  async function handleReset() {
    if (loading || saving || resetting) return;
    if (!window.confirm(t("Are you sure you want to reset the Home Page content? Custom Home content will be removed."))) return;
    setResetting(true); setError(""); setNotice("");
    let resetComplete = false;
    try {
      const result = await adminApi("/admin/website/home/reset", { method: "POST" });
      resetComplete = true;
      setForm(result.data);
      const saved = await adminApi("/admin/website/home");
      setForm(saved);
      setNotice("Home Page content reset successfully.");
    } catch (err) {
      setError(resetComplete ? "Home Page content was reset, but the saved settings could not be reloaded. Refresh to try again." : err instanceof Error ? err.message : "Could not reset Home Page content.");
    } finally { setResetting(false); }
  }

  return (
    <div className="max-w-5xl">
      <div>
        <p className="text-sm font-medium text-blue-600">
          {t("Website Management")}
        </p>

        <h1 className="mt-1 text-3xl font-bold text-slate-900">
          {t("Home Page")}
        </h1>

        <p className="mt-2 text-sm text-slate-500">
          {t("Manage the main hero content displayed on the public website.")}
        </p>
      </div>

      <form
        onSubmit={handleSave}
        className="mt-8 space-y-6 rounded-2xl border border-slate-200 bg-white p-7"
      >
        {loading && <p role="status" className="rounded-xl bg-slate-50 p-3 text-sm text-slate-600">Loading saved home page settings…</p>}
        {error && <p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{error}</p>}
        {notice && <p role="status" className="rounded-xl bg-emerald-50 p-3 text-sm text-emerald-800">{notice}</p>}
        <div className="flex gap-2"><button type="button" onClick={()=>setContentLanguage("en")} className={`rounded-lg px-4 py-2 text-sm font-semibold ${contentLanguage==="en"?"bg-blue-600 text-white":"border"}`}>English (original)</button><button type="button" onClick={()=>setContentLanguage("ar")} className={`rounded-lg px-4 py-2 text-sm font-semibold ${contentLanguage==="ar"?"bg-blue-600 text-white":"border"}`}>العربية</button></div>
        {contentLanguage==="ar"&&<p className="text-sm text-amber-800">Arabic fields are optional. Any empty field keeps using the original English content.</p>}
        <div>
          <label className="mb-2 block text-sm font-semibold text-slate-700">
            {t("Hero Title")}
          </label>

          <input
            name={contentLanguage==="ar"?"ar_hero_title":"hero_title"}
            lang={contentLanguage} dir={contentLanguage==="ar"?"rtl":"ltr"}
            value={contentLanguage==="ar"?(form.translations.ar.hero_title||""):form.hero_title}
            onChange={updateField}
            placeholder={t("Create meaningful connections with digital business cards")}
            className="w-full rounded-xl border border-slate-300 px-4 py-3 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
          />
        </div>

        <div>
          <label className="mb-2 block text-sm font-semibold text-slate-700">
            {t("Hero Subtitle")}
          </label>

          <textarea
            name={contentLanguage==="ar"?"ar_hero_subtitle":"hero_subtitle"}
            lang={contentLanguage} dir={contentLanguage==="ar"?"rtl":"ltr"}
            value={contentLanguage==="ar"?(form.translations.ar.hero_subtitle||""):form.hero_subtitle}
            onChange={updateField}
            rows={4}
            placeholder={t("Share your professional identity instantly using QR, link and NFC.")}
            className="w-full rounded-xl border border-slate-300 px-4 py-3 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
          />
        </div>

        <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
          <div>
            <label className="mb-2 block text-sm font-semibold text-slate-700">
              {t("Primary Button Text")}
            </label>

            <input
              name={contentLanguage==="ar"?"ar_primary_button_text":"primary_button_text"}
              lang={contentLanguage} dir={contentLanguage==="ar"?"rtl":"ltr"}
              value={contentLanguage==="ar"?(form.translations.ar.primary_button_text||""):form.primary_button_text}
              onChange={updateField}
              placeholder={t("Get Started")}
              className="w-full rounded-xl border border-slate-300 px-4 py-3"
            />
          </div>

          <div>
            <label className="mb-2 block text-sm font-semibold text-slate-700">
              {t("Primary Button URL")}
            </label>

            <input
              name="primary_button_url"
              value={form.primary_button_url}
              onChange={updateField}
              placeholder="/signup"
              className="w-full rounded-xl border border-slate-300 px-4 py-3"
            />
          </div>
        </div>

        <div className="grid grid-cols-1 gap-5 md:grid-cols-2">
          <div>
            <label className="mb-2 block text-sm font-semibold text-slate-700">
              {t("Secondary Button Text")}
            </label>

            <input
              name={contentLanguage==="ar"?"ar_secondary_button_text":"secondary_button_text"}
              lang={contentLanguage} dir={contentLanguage==="ar"?"rtl":"ltr"}
              value={contentLanguage==="ar"?(form.translations.ar.secondary_button_text||""):form.secondary_button_text}
              onChange={updateField}
              placeholder={t("View Templates")}
              className="w-full rounded-xl border border-slate-300 px-4 py-3"
            />
          </div>

          <div>
            <label className="mb-2 block text-sm font-semibold text-slate-700">
              {t("Secondary Button URL")}
            </label>

            <input
              name="secondary_button_url"
              value={form.secondary_button_url}
              onChange={updateField}
              placeholder="/templates"
              className="w-full rounded-xl border border-slate-300 px-4 py-3"
            />
          </div>
        </div>

        <div>
          <label className="mb-2 block text-sm font-semibold text-slate-700">
            {t("Hero Image URL")}
          </label>

          <input
            name="hero_image_url"
            value={form.hero_image_url}
            onChange={updateField}
            placeholder="https://..."
            className="w-full rounded-xl border border-slate-300 px-4 py-3"
          />
        </div>

        <div className="flex flex-wrap justify-end gap-3 border-t border-slate-200 pt-6">
          <button
            type="button"
            onClick={handleReset}
            disabled={saving || loading || resetting}
            className="rounded-xl border border-red-200 px-6 py-3 font-semibold text-red-700 hover:bg-red-50 disabled:opacity-60"
          >
            {resetting ? t("Resetting…") : t("Reset Home Page")}
          </button>
          <button
            type="submit"
            disabled={saving || loading || resetting}
            className="rounded-xl bg-blue-600 px-6 py-3 font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
          >
            {saving ? t("Saving…") : t("Save Home Page")}
          </button>
        </div>
      </form>
    </div>
  );
}
