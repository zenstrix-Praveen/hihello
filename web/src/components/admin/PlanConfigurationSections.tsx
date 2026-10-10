"use client";

import NumericStepper from "@/components/admin/NumericStepper";
import { useAdminLocale } from "@/lib/admin-locale";

type Props = {
  limits: Record<string, unknown>;
  features: string[];
  setLimits: (limits: Record<string, unknown>) => void;
  setFeatures: (features: string[]) => void;
};

const usageLimits = [
  ["digital_cards_limit", "Digital Cards", 3],
  ["nfc_tags_limit", "NFC Tags", 2],
  ["qr_codes_limit", "QR Codes", 10],
  ["contacts_limit", "Contacts", 100],
  ["storage_limit", "Storage (GB)", 5],
  ["team_members_limit", "Team Members", 2],
  ["card_templates_limit", "Card Templates", 5],
] as const;

type FeatureRow = { key: string; label: string; unavailable?: boolean };
const featureGroups: { title: string; rows: FeatureRow[] }[] = [
  { title: "Digital Cards & Sharing", rows: [{ key: "custom_qr_branding_enabled", label: "Custom QR Branding", unavailable: true }] },
  { title: "Contacts & Leads", rows: [{ key: "lead_capture_enabled", label: "Lead Capture", unavailable: true }, { key: "contact_export_enabled", label: "Contact Export", unavailable: true }] },
  { title: "Branding & Customization", rows: [{ key: "custom_domain_enabled", label: "Custom Domain" }, { key: "branding_removal_enabled", label: "Remove DBIZCARD Branding" }] },
  { title: "Analytics & Insights", rows: [{ key: "card_views_insights_enabled", label: "Card Views and Insights", unavailable: true }] },
  { title: "Team & Business", rows: [{ key: "team_management_enabled", label: "Team Management", unavailable: true }, { key: "bulk_card_creation_enabled", label: "Bulk Card Creation", unavailable: true }] },
  { title: "Integrations & Support", rows: [{ key: "crm_integration_enabled", label: "CRM Integration", unavailable: true }, { key: "email_signature_enabled", label: "Email Signature", unavailable: true }, { key: "wallet_integration_enabled", label: "Wallet Integration", unavailable: true }, { key: "api_access_enabled", label: "API Access", unavailable: true }, { key: "priority_support_enabled", label: "Priority Support", unavailable: true }] },
];

const supported = new Set(["custom_domain_enabled", "branding_removal_enabled"]);

export default function PlanConfigurationSections({ limits, features, setLimits, setFeatures }: Props) {
  const { t } = useAdminLocale();
  const settings = (limits.feature_settings && typeof limits.feature_settings === "object" ? limits.feature_settings : {}) as Record<string, unknown>;
  const setSetting = (key: string, value: unknown) => setLimits({ ...limits, feature_settings: { ...settings, [key]: value } });
  const setBooleanFeature = (key: string, code: string, checked: boolean) => {
    setSetting(key, checked);
    setFeatures([...new Set([...features.filter(item => item !== code), ...(checked ? [code] : [])])]);
  };

  return <>
    <section className="mt-7" aria-labelledby="usage-limits-heading">
      <div><h3 id="usage-limits-heading" className="text-lg font-bold text-blue-950">{t("3. Usage Limits")}</h3><p className="mt-1 text-sm text-slate-500">{t("Zero means no allowance. Unlimited is saved separately.")}</p></div>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        {usageLimits.map(([key, label, initial]) => {
          const stored = limits[key];
          const value = stored === undefined ? initial : Number(stored);
          const unlimited = limits[`${key.replace(/_limit$/, "")}_unlimited`] === true || value === -1;
          const shown = value === -1 ? initial : value;
          return <div key={key} className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-slate-200 p-3">
            <div className="min-w-28 text-sm font-medium text-slate-800">{label}</div>
            <NumericStepper label={`${label} limit`} value={String(shown)} disabled={unlimited} min={0} max={2147483647} onChange={next => setLimits({ ...limits, [key]: Number(next), [`${key.replace(/_limit$/, "")}_unlimited`]: false })} />
            <label className="flex min-h-10 items-center gap-2 text-sm"><input type="checkbox" checked={unlimited} onChange={event => setLimits({ ...limits, [key]: shown, [`${key.replace(/_limit$/, "")}_unlimited`]: event.target.checked })} className="h-4 w-4" />Unlimited</label>
          </div>;
        })}
      </div>
    </section>

    <section className="mt-7" aria-labelledby="included-features-heading">
      <h3 id="included-features-heading" className="text-lg font-bold text-blue-950">{t("4. Included Features")}</h3>
      <div className="mt-3 space-y-4">
        {featureGroups.map(group => <section key={group.title} aria-label={group.title} className="rounded-xl border border-slate-200 p-4">
          <h4 className="font-semibold text-slate-800">{t(group.title)}</h4>
          {group.title === "Analytics & Insights" && <div className="mt-3 rounded-lg bg-slate-50 p-3">
          <label htmlFor="plan-analytics-level" className="text-sm font-medium">{t("Analytics")}</label>
          <select id="plan-analytics-level" value={String(limits.analytics_level ?? (features.includes("analytics") ? "basic" : "disabled"))} onChange={event => { const level = event.target.value; setLimits({ ...limits, analytics_level: level }); setFeatures([...new Set([...features.filter(item => item !== "analytics"), ...(level === "disabled" ? [] : ["analytics"])])]); }} className="mt-2 w-full rounded-lg border border-slate-300 px-3 py-2">
            <option value="disabled">{t("Disabled")}</option><option value="basic">{t("Basic")}</option><option value="advanced" disabled>{t("Advanced (unavailable)")}</option>
          </select>
          <p className="mt-1 text-xs text-slate-500">{t("Advanced analytics is unavailable until its reporting capabilities are implemented.")}</p>
          </div>}
          {group.title === "Branding & Customization" && <div className="mt-3 rounded-lg bg-slate-50 p-3">
          <label htmlFor="plan-branding-level" className="text-sm font-medium">{t("Branding")}</label>
          <select id="plan-branding-level" value={String(limits.branding_level ?? (features.includes("custom_branding") ? "custom" : "standard"))} onChange={event => { const level = event.target.value; setLimits({ ...limits, branding_level: level }); setFeatures([...new Set([...features.filter(item => item !== "custom_branding"), ...(level === "custom" ? ["custom_branding"] : [])])]); }} className="mt-2 w-full rounded-lg border border-slate-300 px-3 py-2"><option value="standard">{t("Standard")}</option><option value="custom">{t("Custom")}</option></select>
          </div>}
          <div className="mt-3 grid gap-3 sm:grid-cols-2">{group.rows.map(row => {
          const code = row.key === "custom_domain_enabled" ? "custom_domain" : row.key === "branding_removal_enabled" ? "branding_removal" : row.key;
          const checked = supported.has(row.key) ? features.includes(code) : settings[row.key] === true;
          return <label key={row.key} className={`flex min-h-14 items-center justify-between gap-3 rounded-xl border border-slate-200 p-4 text-sm ${row.unavailable ? "bg-slate-50 text-slate-500" : ""}`}>
            <span><span className="block font-medium">{t(row.label)}</span>{row.unavailable && <span className="text-xs">{t("Unavailable until implemented")}</span>}</span>
            <input type="checkbox" checked={checked} disabled={row.unavailable} onChange={event => supported.has(row.key) ? setBooleanFeature(row.key, code, event.target.checked) : setSetting(row.key, event.target.checked)} className="h-5 w-5" aria-label={row.label} />
          </label>;
          })}</div>
        </section>)}
      </div>
    </section>
  </>;
}
