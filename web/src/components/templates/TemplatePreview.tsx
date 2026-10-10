"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import CardPreview from "@/components/cards/CardPreview";
import type { DesignSettings } from "@/components/dashboard/data";

export type TemplatePreviewData = {
  id: number;
  name: string;
  slug?: string;
  category?: string;
  description?: string;
  preview_image_url?: string;
  design_settings?: Partial<DesignSettings>;
};

const base: DesignSettings = {
  theme: "ocean-blue", primary_color: "#2563EB", secondary_color: "#0EA5E9", background_color: "#FFFFFF", text_color: "#0F172A", accent_color: "#06B6D4",
  font_family: "System Sans", font_scale: 1, layout: "classic", button_style: "rounded", border_radius: "large", profile_style: "circle", logo_position: "top-right", social_icon_style: "pills", section_order: ["about", "contact", "social", "links", "videos", "qr"],
};

const styles: Record<string, Partial<DesignSettings>> = {
  "modern-blue": { theme: "ocean-blue", primary_color: "#2563EB", secondary_color: "#0EA5E9", background_color: "#FFFFFF", text_color: "#0F172A", accent_color: "#2563EB", layout: "classic", profile_style: "circle", button_style: "pill" },
  "clean-minimal": { theme: "monochrome", primary_color: "#57534E", secondary_color: "#A8A29E", background_color: "#FAFAF9", text_color: "#1C1917", accent_color: "#D6B98C", font_family: "Georgia", layout: "minimal", profile_style: "square", button_style: "square", social_icon_style: "monochrome" },
  "creative-dark": { theme: "midnight", primary_color: "#7C3AED", secondary_color: "#EC4899", background_color: "#080B14", text_color: "#FFFFFF", accent_color: "#2563EB", layout: "hero", profile_style: "rounded", button_style: "pill", social_icon_style: "branded" },
  "corporate-pro": { theme: "royal-indigo", primary_color: "#0B2A5B", secondary_color: "#1D4ED8", background_color: "#F8FAFC", text_color: "#0F172A", accent_color: "#38BDF8", layout: "executive", profile_style: "square", button_style: "rounded" },
  "personal-vibe": { theme: "rose", primary_color: "#EC4899", secondary_color: "#F43F5E", background_color: "#FFF1F2", text_color: "#4A1730", accent_color: "#FB7185", layout: "centered", profile_style: "circle", button_style: "pill", social_icon_style: "branded" },
  "colorful-pop": { theme: "sunset", primary_color: "#DB2777", secondary_color: "#7C3AED", background_color: "#FFF7ED", text_color: "#312E4B", accent_color: "#06B6D4", layout: "hero", profile_style: "circle", button_style: "pill", social_icon_style: "branded" },
  "executive-gold": { theme: "emerald", primary_color: "#064E3B", secondary_color: "#D4A84F", background_color: "#10251F", text_color: "#FFF7E6", accent_color: "#D4A84F", font_family: "Georgia", layout: "split", profile_style: "square", button_style: "square", social_icon_style: "monochrome" },
  "tech-developer": { theme: "midnight", primary_color: "#0F172A", secondary_color: "#6D28D9", background_color: "#111827", text_color: "#F8FAFC", accent_color: "#2563EB", font_family: "Trebuchet MS", layout: "compact", profile_style: "rounded", button_style: "square", social_icon_style: "branded" },
  "soft-gradient": { theme: "violet", primary_color: "#8B5CF6", secondary_color: "#E879F9", background_color: "#F5F3FF", text_color: "#312E4B", accent_color: "#60A5FA", layout: "hero", profile_style: "circle", button_style: "pill", social_icon_style: "branded" },
  "warm-modern": { theme: "sunset", primary_color: "#EA580C", secondary_color: "#9A3412", background_color: "#FFF7ED", text_color: "#431407", accent_color: "#FDBA74", font_family: "Georgia", layout: "minimal", profile_style: "rounded", button_style: "rounded" },
  "creative-portfolio": { theme: "teal", primary_color: "#0F766E", secondary_color: "#0891B2", background_color: "#F0FDFA", text_color: "#134E4A", accent_color: "#15803D", layout: "centered", profile_style: "rounded", button_style: "pill", social_icon_style: "branded" },
};

const sample = {
  name: "Alex Johnson", job_title: "Product Manager", company_name: "TechNova Inc.", email: "alex@example.test",
  website: "https://technova.example", address: "San Francisco", social_links: [{ label: "LinkedIn", url: "https://linkedin.example/alex" }],
  custom_links: [{ label: "Portfolio", url: "https://technova.example/portfolio" }], template_name: "",
};

function designFor(template: TemplatePreviewData): DesignSettings {
  const key = (template.slug || template.name).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
  const categoryKey = (template.category || "").toLowerCase();
  const categoryPreset = categoryKey === "minimal" || categoryKey === "luxury" ? styles["clean-minimal"]
    : categoryKey === "dark" || categoryKey === "tech" ? styles["creative-dark"]
      : categoryKey === "creative" || categoryKey === "colorful" ? styles["personal-vibe"]
        : categoryKey === "corporate" || categoryKey === "professional" ? styles["corporate-pro"]
          : categoryKey === "gradient" || categoryKey === "modern" ? styles["soft-gradient"] : styles["modern-blue"];
  // Backend settings are authoritative. Presets only keep older records with empty
  // settings attractive and remain within the design options accepted by the API.
  return { ...base, ...(styles[key] || categoryPreset), ...(template.design_settings || {}) };
}

export default function TemplatePreview({ template, large = false }: { template: TemplatePreviewData; large?: boolean }) {
  const { t } = useCustomerLocale();
  const design = designFor(template);
  const palette = `radial-gradient(circle at 15% 15%, ${design.accent_color}55, transparent 36%), linear-gradient(135deg, ${design.primary_color}22, ${design.secondary_color}33)`;
  return <div className={`group/preview relative flex w-full items-center justify-center overflow-hidden ${large ? "min-h-[600px] rounded-2xl p-8" : "h-[370px] px-4 py-3"}`} style={{ background: palette }}>
    <div className={`w-full max-w-sm transition-transform duration-300 group-hover/preview:scale-[1.02] ${large ? "" : "scale-[0.76]"}`}>
      <CardPreview card={{ ...sample, name: t(sample.name), job_title: t(sample.job_title), company_name: t(sample.company_name), address: t(sample.address), custom_links: sample.custom_links.map(link => ({ ...link, label: t(link.label) })), design_settings: design, template_name: template.name }} />
    </div>
    <span className="absolute start-4 top-4 rounded-full border border-white/70 bg-white/85 px-3 py-1 text-[10px] font-bold uppercase tracking-[.16em] text-slate-700 shadow-sm">{t(template.category || "Dbizcard design")}</span>
  </div>;
}

export { designFor };
