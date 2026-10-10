"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import type { CSSProperties } from "react";
import type { DesignSettings } from "@/components/dashboard/data";
import { ContactRound, Globe2, Mail, MapPin, MessageCircle, Phone } from "lucide-react";

type PreviewCard = {
  name?: string; job_title?: string; company_name?: string; phone?: string; email?: string;
  website?: string; address?: string; photo_url?: string; logo_url?: string; primary_color?: string;
  design_settings?: Partial<DesignSettings>;
  social_links?: { label: string; url: string }[]; custom_links?: { label: string; url: string }[]; videos?: { label: string; url: string }[]; template_name?: string;
};
const defaults: DesignSettings = { theme: "ocean-blue", primary_color: "#2563EB", secondary_color: "#4F46E5", background_color: "#FFFFFF", text_color: "#0F172A", accent_color: "#06B6D4", font_family: "System Sans", font_scale: 1, layout: "classic", button_style: "rounded", border_radius: "large", profile_style: "circle", logo_position: "top-right", social_icon_style: "pills", section_order: ["about", "contact", "social", "links", "videos", "qr"] };
const radius: Record<string, string> = { small: "rounded-xl", medium: "rounded-2xl", large: "rounded-3xl" };
const buttonRadius: Record<string, string> = { rounded: "rounded-xl", pill: "rounded-full", square: "rounded-md" };
const profileRadius: Record<string, string> = { circle: "rounded-full", rounded: "rounded-2xl", square: "rounded-none" };
const fontStacks: Record<string, string> = { "System Sans": "Inter, ui-sans-serif, system-ui, sans-serif", Arial: "Arial, Helvetica, sans-serif", "Trebuchet MS": "'Trebuchet MS', sans-serif", Georgia: "Georgia, serif" };

/** Shared live card renderer used by gallery, editor, dashboard preview and public card styling data. */
export default function CardPreview({ card }: { card: PreviewCard }) {
  const { t } = useCustomerLocale();
  const design = { ...defaults, ...card.design_settings, primary_color: card.design_settings?.primary_color || card.primary_color || defaults.primary_color };
  const style = { "--card-primary": design.primary_color, "--card-secondary": design.secondary_color, "--card-accent": design.accent_color, "--card-background": design.background_color, "--card-text": design.text_color, "--card-font-scale": design.font_scale, fontFamily: fontStacks[design.font_family] || fontStacks["System Sans"] } as CSSProperties;
  const safeUrl = (value?: string) => value && /^https?:\/\//i.test(value) ? value : undefined;
  const layout = design.layout;
  const centered = layout === "centered";
  const minimal = layout === "minimal";
  const compact = layout === "compact";
  const executive = layout === "executive";
  const split = layout === "split";
  const hero = layout === "hero";
  const channels = design.background_color.match(/^#([0-9a-f]{6})$/i)?.[1];
  const dark = ["midnight", "creative-dark"].includes(design.theme) || (channels ? (parseInt(channels.slice(0, 2), 16) * 299 + parseInt(channels.slice(2, 4), 16) * 587 + parseInt(channels.slice(4, 6), 16) * 114) / 1000 < 128 : false);
  const profile = profileRadius[design.profile_style] || profileRadius.circle;
  const button = buttonRadius[design.button_style] || buttonRadius.rounded;
  const subdued = dark ? "border-white/15 bg-white/10 text-white" : "border-slate-200 bg-white/70 text-slate-700";
  const socials = (card.social_links || []).filter(x => safeUrl(x.url));
  const links = (card.custom_links || []).filter(x => safeUrl(x.url));

  return <article style={style} className={`relative mx-auto w-full max-w-sm overflow-hidden ${radius[design.border_radius] || radius.large} border shadow-xl ${dark ? "border-white/10" : "border-slate-200"}`}>
    <header className={`relative overflow-hidden ${minimal ? "min-h-32 border-b border-slate-200 px-6 pb-5 pt-7" : executive ? "min-h-40 px-6 pb-6 pt-7" : split ? "min-h-36 px-5 pb-5 pt-6" : centered ? "min-h-40 px-6 pb-5 pt-7 text-center" : hero ? "min-h-44 px-6 pb-6 pt-7" : "min-h-36 px-6 pb-5 pt-6"}`} style={{ background: minimal ? "#fff" : executive ? `linear-gradient(115deg, ${design.primary_color} 0 78%, ${design.secondary_color} 78%)` : split ? `linear-gradient(135deg, ${design.background_color}, ${design.background_color} 60%, ${design.secondary_color})` : hero ? `radial-gradient(circle at 85% 15%, ${design.accent_color}, transparent 35%), linear-gradient(135deg, ${design.primary_color}, ${design.secondary_color})` : `linear-gradient(135deg, ${design.primary_color}, ${design.secondary_color})`, color: minimal || split ? design.text_color : "white" }}>
      {hero && <><div className="absolute -right-8 -top-12 h-36 w-36 rounded-full border-[22px] border-white/10"/><div className="absolute -bottom-16 left-1/3 h-32 w-32 rounded-full bg-white/10 blur-xl"/></>}
      {split && <div className="absolute left-0 top-0 h-full w-2" style={{ backgroundColor: design.accent_color }}/>} 
      <div className={`relative z-10 flex items-end gap-4 ${centered ? "flex-col items-center" : split ? "flex-row-reverse justify-between" : ""}`}>
        <div className={`grid h-[68px] w-[68px] shrink-0 place-items-center overflow-hidden ${profile} border-[3px] ${minimal ? "border-slate-200 bg-slate-100" : "border-white/80 bg-white/85"} text-xl font-bold text-slate-700 shadow-lg`}>
          {safeUrl(card.photo_url) ? <img src={safeUrl(card.photo_url)} alt={t("Profile")} className="h-full w-full object-cover"/> : (card.name?.[0] || "A").toUpperCase()}
        </div>
        {safeUrl(card.logo_url) && design.logo_position === "top-right" && <img src={safeUrl(card.logo_url)} alt={t("Company logo")} className="ms-auto max-h-9 max-w-24 rounded-lg bg-white p-1.5 object-contain"/>}
        {executive && <span className="ms-auto rounded-sm border border-white/40 px-2 py-1 text-[9px] font-bold uppercase tracking-[.18em] text-white/90">{t("Company")}</span>}
      </div>
      {hero && <span className="relative z-10 mt-4 inline-flex rounded-full border border-white/30 bg-white/15 px-3 py-1 text-[10px] font-semibold text-white backdrop-blur">{t("Digital identity")}</span>}
    </header>

    <div className={`p-5 ${centered ? "text-center" : ""} ${dark ? "text-white" : ""}`} style={{ backgroundColor: dark ? design.background_color : minimal ? "#FFFFFF" : design.background_color, color: dark ? design.text_color : design.text_color }}>
      {safeUrl(card.logo_url) && design.logo_position === "before-name" && <img src={safeUrl(card.logo_url)} alt={t("Company logo")} className={`${centered ? "mx-auto" : ""} mb-3 max-h-9 max-w-28 object-contain`}/>}
      <p className={`text-[calc(1.42rem*var(--card-font-scale))] font-bold leading-tight ${design.font_family === "Georgia" ? "tracking-tight" : "tracking-[-.03em]"}`}>{card.name || t("Your Name")}</p>
      <p className={`mt-1 text-[calc(.78rem*var(--card-font-scale))] ${dark ? "text-white/70" : "text-slate-500"}`}>{[card.job_title, card.company_name].filter(Boolean).join(" · ") || t("Job title · Company")}</p>
      {executive && <div className="mt-3 h-px w-12" style={{ backgroundColor: design.accent_color }}/>} 
      {safeUrl(card.logo_url) && design.logo_position === "below-name" && <img src={safeUrl(card.logo_url)} alt={t("Company logo")} className={`${centered ? "mx-auto" : ""} mt-3 max-h-9 max-w-28 object-contain`}/>}

      <div className={`mt-5 grid ${compact ? "grid-cols-1" : "grid-cols-2"} gap-2`}>
        {card.phone && <span className={`${button} flex items-center justify-center gap-1.5 px-2 py-2 text-xs font-semibold text-white`} style={{ backgroundColor: design.primary_color }}><Phone size={14}/>{t("Call")}</span>}
        {card.phone && <span className={`${button} flex items-center justify-center gap-1.5 border px-2 py-2 text-xs font-semibold ${subdued}`}><MessageCircle size={14}/>{t("WhatsApp")}</span>}
        {card.email && <span className={`${button} flex items-center justify-center gap-1.5 border px-2 py-2 text-xs font-semibold ${subdued}`}><Mail size={14}/>{t("Email")}</span>}
        {safeUrl(card.website) && <span className={`${button} flex items-center justify-center gap-1.5 border px-2 py-2 text-xs font-semibold ${subdued}`}><Globe2 size={14}/>{t("Website")}</span>}
        {card.address && <span className={`${button} col-span-2 flex items-center justify-center gap-1.5 border px-2 py-2 text-xs font-semibold ${subdued}`}><MapPin size={14}/>{t("Location")}</span>}
      </div>
      {!!socials.length && <div className={`mt-4 flex flex-wrap gap-2 ${centered ? "justify-center" : ""}`}>{socials.map((item, i) => <span key={`${item.label}-${i}`} className={`${design.social_icon_style === "pills" ? "rounded-full" : "rounded-lg"} px-3 py-1.5 text-xs font-semibold`} style={{ backgroundColor: dark ? `${design.accent_color}30` : design.social_icon_style === "branded" ? `${design.accent_color}20` : "#F1F5F9", color: dark ? design.text_color : design.primary_color }}>{item.label}</span>)}</div>}
      {!!links.length && <div className="mt-4 space-y-2">{links.slice(0, 2).map((item, i) => <div key={`${item.label}-${i}`} className={`${button} flex items-center justify-between border px-3 py-2.5 text-xs font-semibold ${subdued}`}><span>{item.label || t("Portfolio")}</span><span aria-hidden="true">↗</span></div>)}</div>}
      <div className={`mt-4 flex items-center justify-center gap-2 ${button} px-4 py-2.5 text-sm font-bold text-white shadow-sm`} style={{ backgroundColor: design.primary_color }}><ContactRound size={16}/>{t("Save Contact")}</div>
      <p className={`mt-4 border-t pt-3 text-[10px] ${dark ? "border-white/10 text-white/50" : "border-slate-200/70 text-slate-500"}`}>{t(card.template_name || design.layout)}{t("· Dbizcard preview")}</p>
    </div>
  </article>;
}
