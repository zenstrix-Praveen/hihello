"use client";

import { useState, type CSSProperties, type ReactNode } from "react";
import type { DesignSettings } from "@/components/dashboard/data";
import { BriefcaseBusiness, Building2, ContactRound, Globe2, Mail, MapPin, MessageCircle, Phone, QrCode, Share2 } from "lucide-react";
import { useCustomerLocale } from "@/lib/customer-locale";

export type PublicCardData = {
  id: number; public_id?: string; name: string; slug: string; job_title: string; company_name: string;
  phone: string; email: string; website: string; address: string; photo_url: string; logo_url: string;
  primary_color: string; template_id?: number; template_name?: string; template_preview_image_url?: string;
  design_settings?:Partial<DesignSettings>;
  published_at?: string; social_links: { label: string; url: string }[];
  custom_links: { label: string; url: string }[]; videos: { label: string; url: string }[];
};

function safeWebURL(value?: string) {
  if (!value) return "";
  try { const u = new URL(value); return u.protocol === "http:" || u.protocol === "https:" ? u.toString() : ""; }
  catch { return ""; }
}

export default function PublicCard({ card, apiBase }: { card: PublicCardData; apiBase: string }) {
  const {t}=useCustomerLocale();
  const [shareMessage, setShareMessage] = useState("");
  const [qrError, setQRError] = useState(false);
  const defaults:DesignSettings={theme:"ocean-blue",primary_color:"#2563EB",secondary_color:"#4F46E5",background_color:"#FFFFFF",text_color:"#0F172A",accent_color:"#06B6D4",font_family:"Inter",font_scale:1,layout:"classic",button_style:"rounded",border_radius:"large",profile_style:"circle",logo_position:"top-right",social_icon_style:"pills",section_order:["about","contact","social","links","videos","qr"]};
  const design={...defaults,...card.design_settings,primary_color:card.design_settings?.primary_color||(/^#[0-9a-fA-F]{6}$/.test(card.primary_color||"")?card.primary_color:defaults.primary_color)};
  const color=design.primary_color;
  const radius:Record<string,string>={small:"rounded-2xl",medium:"rounded-3xl",large:"rounded-[2rem]"};
  const profileRadius:Record<string,string>={circle:"rounded-full",rounded:"rounded-2xl",square:"rounded-none"};
  const buttonRadius:Record<string,string>={rounded:"rounded-xl",pill:"rounded-full",square:"rounded-md"};
  const fontStacks:Record<string,string>={"System Sans":"Inter, ui-sans-serif, system-ui, sans-serif",Arial:"Arial, Helvetica, sans-serif","Trebuchet MS":"'Trebuchet MS', sans-serif",Georgia:"Georgia, serif"};
  const cardStyle={"--card-color":color,"--card-secondary":design.secondary_color,"--card-accent":design.accent_color,"--card-font-scale":design.font_scale,fontFamily:fontStacks[design.font_family]||fontStacks["System Sans"]} as CSSProperties;
  const isCentered=design.layout==="centered";
  const isMinimal=design.layout==="minimal";
  const isExecutive=design.layout==="executive";
  const isHero=design.layout==="hero";
  const isSplit=design.layout==="split";
  const relativeURL = `/c/${encodeURIComponent(card.slug)}`;
  const getPublicURL = () => `${window.location.origin}${relativeURL}`;
  const phone = (card.phone || "").trim();
  const phoneHref = phone ? `tel:${phone.replace(/[^+\d]/g, "")}` : "";
  const whatsappNumber = phone.replace(/\D/g, "");
  const whatsappHref = whatsappNumber.length >= 7 ? `https://wa.me/${whatsappNumber}` : "";
  const website = safeWebURL(card.website);
  const event = (action: string, type = "link_click") => {
    void fetch(`${apiBase}/public/cards/${encodeURIComponent(card.slug)}/events`, {
      method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ type, action }), keepalive: true,
    }).catch(() => undefined);
  };
  async function share() {
    event("share");
    try {
      const url = getPublicURL();
      if (navigator.share) { await navigator.share({ title: `${card.name} | ${t("Digital Business Card")}`, url }); return; }
      if (navigator.clipboard?.writeText) { await navigator.clipboard.writeText(url); setShareMessage(t("Link copied to clipboard.")); return; }
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      try { await navigator.clipboard.writeText(getPublicURL()); setShareMessage(t("Link copied to clipboard.")); return; } catch { /* Show manual copy fallback below. */ }
    }
    setShareMessage(t("Copy this link to share the card."));
  }
  const socials = (card.social_links || []).map(item => ({ ...item, href: safeWebURL(item.url) })).filter(item => item.href);
  const customLinks = (card.custom_links || []).map(item => ({ ...item, href: safeWebURL(item.url) })).filter(item => item.href);
  const videos = (card.videos || []).map(item => ({ ...item, href: safeWebURL(item.url) })).filter(item => item.href);
  const mapURL = card.address ? `https://maps.google.com/?q=${encodeURIComponent(card.address)}` : "";
  const sectionNodes:Record<string,ReactNode>={
    about:(card.job_title||card.company_name)?<section key="about" className="mt-8 border-t border-slate-100/60 pt-6"><h2 className="text-sm font-bold uppercase tracking-wide opacity-60">{t("About")}</h2><p className="mt-3 leading-7 opacity-85">{[card.job_title,card.company_name].filter(Boolean).join(` ${t("at")} `)}</p></section>:null,
    contact:(phone||card.email||website||card.address)?<section key="contact" className="mt-8 border-t border-slate-100/60 pt-6"><h2 className="text-sm font-bold uppercase tracking-wide opacity-60">{t("Contact Information")}</h2><dl className="mt-4 space-y-4">{phone&&<div><dt className="text-xs font-semibold opacity-50">{t("Phone")}</dt><dd dir="ltr" className="mt-1 break-words text-sm font-medium">{phone}</dd></div>}{card.email&&<div><dt className="text-xs font-semibold opacity-50">{t("Email")}</dt><dd dir="ltr" className="mt-1 break-words text-sm font-medium">{card.email}</dd></div>}{website&&<div><dt className="text-xs font-semibold opacity-50">{t("Website")}</dt><dd dir="ltr" className="mt-1 break-all text-sm font-medium">{website}</dd></div>}{card.address&&<div><dt className="text-xs font-semibold opacity-50">{t("Address")}</dt><dd className="mt-1 whitespace-pre-line text-sm font-medium">{card.address}</dd></div>}</dl></section>:null,
    social:socials.length?<section key="social" className="mt-8 border-t border-slate-100/60 pt-6"><h2 className="text-sm font-bold uppercase tracking-wide opacity-60">{t("Connect")}</h2><div className="mt-4 flex flex-wrap gap-2">{socials.map((item,i)=><a key={`${item.label}-${i}`} onClick={()=>event("social")} href={item.href} target="_blank" rel="noopener noreferrer" className={`${design.social_icon_style==="pills"?"rounded-full":"rounded-lg"} px-4 py-2 text-sm font-semibold`} style={{backgroundColor:design.social_icon_style==="branded"?`${design.accent_color}22`:"#F1F5F9",color:color}}>{item.label}</a>)}</div></section>:null,
    links:customLinks.length?<section key="links" className="mt-8 border-t border-slate-100/60 pt-6"><h2 className="text-sm font-bold uppercase tracking-wide opacity-60">{t("More Links")}</h2><div className="mt-4 space-y-2">{customLinks.map((item,i)=><a key={`${item.label}-${i}`} onClick={()=>event("custom")} href={item.href} target="_blank" rel="noopener noreferrer" className="flex items-center justify-between rounded-xl border border-slate-200/70 p-4 text-sm font-semibold"><span>{item.label||t("Open link")}</span><Globe2 size={16} style={{color}}/></a>)}</div></section>:null,
    videos:videos.length?<section key="videos" className="mt-8 border-t border-slate-100/60 pt-6"><h2 className="text-sm font-bold uppercase tracking-wide opacity-60">{t("Videos")}</h2><div className="mt-4 space-y-2">{videos.map((item,i)=><a key={`${item.label}-${i}`} onClick={()=>event("video")} href={item.href} target="_blank" rel="noopener noreferrer" className="flex items-center justify-between rounded-xl border border-slate-200/70 p-4 text-sm font-semibold"><span>{item.label||t("Watch video")}</span><span style={{color}}>{t("Watch")} ↗</span></a>)}</div></section>:null,
    qr:<details key="qr" className="mt-8 border-t border-slate-100/60 pt-5"><summary onClick={()=>event("qr")} className="flex cursor-pointer list-none items-center justify-center gap-2 rounded-xl border border-slate-200/70 px-4 py-3 text-sm font-semibold"><QrCode size={18}/>{t("Show QR")}</summary><div className="py-4 text-center">{qrError?<p className="text-sm opacity-60">{t("No active QR code is available for this card.")}</p>:<img onError={()=>setQRError(true)} src={`${apiBase}/public/cards/${encodeURIComponent(card.slug)}/qr`} alt={t("QR code for this digital card")} className="mx-auto h-56 w-56 rounded-xl border border-slate-100 bg-white p-2"/>}</div></details>
  };

  return <main className="min-h-screen bg-gradient-to-b from-blue-50 via-slate-50 to-white px-4 py-8 sm:px-6 sm:py-14">
    <article style={cardStyle} className={`mx-auto w-full max-w-xl overflow-hidden ${radius[design.border_radius]||radius.large} border border-slate-200 bg-white shadow-xl shadow-slate-900/5`}>
      <header className={`relative flex min-h-44 items-end overflow-hidden px-7 pb-7 pt-10 sm:px-10 ${isCentered?"justify-center":isSplit?"justify-end":""}`} style={{ background:isMinimal?design.background_color:isExecutive?`linear-gradient(115deg, ${color} 0 78%, ${design.secondary_color} 78%)`:isHero?`radial-gradient(circle at 82% 18%, ${design.accent_color}, transparent 34%), linear-gradient(130deg, ${color}, ${design.secondary_color})`:isSplit?`linear-gradient(135deg, ${design.background_color}, ${design.background_color} 62%, ${design.secondary_color})`:`linear-gradient(130deg, ${color}, ${design.secondary_color})`,color:isMinimal||isSplit?design.text_color:"#fff",borderBottom:isMinimal?"1px solid #e2e8f0":undefined }}>
        {isSplit&&<div className="absolute inset-y-0 left-0 w-2" style={{backgroundColor:design.accent_color}}/>}
        {isHero&&<><div className="absolute -right-10 -top-12 h-44 w-44 rounded-full border-[24px] border-white/10"/><div className="absolute -bottom-16 left-1/3 h-32 w-32 rounded-full bg-white/10 blur-xl"/></>}
        {safeWebURL(card.template_preview_image_url) && <img src={safeWebURL(card.template_preview_image_url)} alt="" aria-hidden="true" className="absolute inset-0 h-full w-full object-cover opacity-20" />}
        <div className="absolute inset-0 bg-gradient-to-br from-black/10 to-black/40" />
        <div className={`relative z-10 flex w-full items-end gap-4 ${isCentered?"flex-col items-center text-center":isSplit?"flex-row-reverse justify-between":""}`}>
          <div className={`grid h-24 w-24 shrink-0 place-items-center overflow-hidden ${profileRadius[design.profile_style]||profileRadius.circle} border-4 border-white/80 bg-white/20 text-3xl font-bold shadow-lg`}>{safeWebURL(card.photo_url) ? <img src={safeWebURL(card.photo_url)} alt={`${card.name} profile`} className="h-full w-full object-cover" /> : card.name.trim().slice(0, 1).toUpperCase()}</div>
          {safeWebURL(card.logo_url) && design.logo_position==="top-right" && <img src={safeWebURL(card.logo_url)} alt={`${card.company_name || "Company"} logo`} className="ml-auto max-h-12 max-w-28 rounded-lg bg-white/95 p-2 object-contain" />}
        </div>
      </header>
      <div className={`p-7 sm:p-10 ${isCentered?"text-center":""}`} style={{backgroundColor:design.background_color,color:design.text_color}}>
        <div>{safeWebURL(card.logo_url)&&design.logo_position==="before-name"&&<img src={safeWebURL(card.logo_url)} alt={`${card.company_name||"Company"} logo`} className="mb-3 max-h-12 max-w-32 object-contain"/>}<p className="text-xs font-bold uppercase tracking-[.2em]" style={{color}}>{t("Digital Business Card")}</p><h1 className="mt-2 break-words text-[calc(2rem*var(--card-font-scale))] font-black tracking-tight sm:text-[calc(2.25rem*var(--card-font-scale))]">{card.name}</h1>
          {(card.job_title || card.company_name) && <p className="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-base text-slate-600">{card.job_title && <><BriefcaseBusiness size={17} />{card.job_title}</>}{card.job_title && card.company_name && <span aria-hidden="true">·</span>}{card.company_name && <><Building2 size={17} />{card.company_name}</>}</p>}
          {safeWebURL(card.logo_url)&&design.logo_position==="below-name"&&<img src={safeWebURL(card.logo_url)} alt={`${card.company_name||"Company"} logo`} className="mt-3 max-h-12 max-w-32 object-contain"/>}
        </div>

        <section aria-label={t("Contact actions")} className="mt-7 grid grid-cols-2 gap-3">
          {phoneHref && <a onClick={() => event("call")} href={phoneHref} className={`flex items-center justify-center gap-2 ${buttonRadius[design.button_style]||buttonRadius.rounded} px-4 py-3.5 text-sm font-bold text-white shadow-sm`} style={{ backgroundColor: color }}><Phone size={18} />{t("Call")}</a>}
          {whatsappHref && <a onClick={() => event("whatsapp")} href={whatsappHref} target="_blank" rel="noopener noreferrer" className={`flex items-center justify-center gap-2 ${buttonRadius[design.button_style]||buttonRadius.rounded} border border-emerald-200 bg-emerald-50 px-4 py-3.5 text-sm font-bold text-emerald-800`}><MessageCircle size={18} />{t("WhatsApp")}</a>}
          {card.email && <a onClick={() => event("email")} href={`mailto:${encodeURIComponent(card.email)}`} className={`flex items-center justify-center gap-2 ${buttonRadius[design.button_style]||buttonRadius.rounded} border border-slate-200 px-4 py-3.5 text-sm font-bold text-slate-800`}><Mail size={18} />{t("Email")}</a>}
          {website && <a onClick={() => event("website")} href={website} target="_blank" rel="noopener noreferrer" className={`flex items-center justify-center gap-2 ${buttonRadius[design.button_style]||buttonRadius.rounded} border border-slate-200 px-4 py-3.5 text-sm font-bold text-slate-800`}><Globe2 size={18} />{t("Visit Website")}</a>}
          {mapURL && <a onClick={() => event("map")} href={mapURL} target="_blank" rel="noopener noreferrer" className={`flex items-center justify-center gap-2 ${buttonRadius[design.button_style]||buttonRadius.rounded} border border-slate-200 px-4 py-3.5 text-sm font-bold text-slate-800`}><MapPin size={18} />{t("Map")}</a>}
          <a onClick={() => event("save_contact", "contact_exchange")} href={`${apiBase}/public/cards/${encodeURIComponent(card.slug)}/vcard`} className={`flex items-center justify-center gap-2 ${buttonRadius[design.button_style]||buttonRadius.rounded} border border-slate-200 px-4 py-3.5 text-sm font-bold text-slate-800`}><ContactRound size={18} />{t("Save Contact")}</a>
          <button onClick={share} type="button" className={`flex items-center justify-center gap-2 ${buttonRadius[design.button_style]||buttonRadius.rounded} border border-slate-200 px-4 py-3.5 text-sm font-bold text-slate-800`}><Share2 size={18} />{t("Share")}</button>
        </section>
        {shareMessage && <div className="mt-3 rounded-xl bg-blue-50 p-3 text-sm text-blue-800" role="status"><p>{shareMessage}</p>{shareMessage.includes("Copy this") && <div className="mt-2 flex gap-2"><input aria-label={t("Public card URL")} dir="ltr" readOnly value={getPublicURL()} className="min-w-0 flex-1 rounded-lg border border-blue-200 bg-white px-3 py-2 text-xs" /><button onClick={() => { void navigator.clipboard?.writeText(getPublicURL()).then(() => setShareMessage(t("Link copied to clipboard."))).catch(() => undefined); }} className="rounded-lg bg-blue-600 px-3 py-2 text-xs font-semibold text-white">{t("Copy")}</button></div>}</div>}

        {design.section_order.map(name=>sectionNodes[name]).filter(Boolean)}
        <footer className="mt-8 border-t border-slate-100 pt-5 text-center text-xs opacity-60">{card.template_name ? `${t(card.template_name)} · ` : ""}{t("Digital business card")}</footer>
      </div>
    </article>
  </main>;
}
