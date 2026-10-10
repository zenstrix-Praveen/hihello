"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { visualFeatures } from "./visualFeatures";
import FeatureIllustration from "./FeatureIllustration";
import { useCustomerLocale } from "@/lib/customer-locale";
import {
  BarChart3,
  Building2,
  ContactRound,
  Globe2,
  Mail,
  MapPin,
  MessageCircle,
  Nfc,
  Phone,
  QrCode,
  Share2,
  UserRound,
  X,
  type LucideIcon,
} from "lucide-react";

export type FeatureDetail = {
  id: string;
  title: string;
  description: string;
  howItWorks: string;
  examples: string[];
  icon: LucideIcon;
  cardDescription: string;
  shortDescription?: string;
  heading?: string;
  illustration?: string;
  illustrationAlt?: string;
  steps?: string[];
  benefits?: string[];
  ctaLabel?: string;
  ctaHref?: string;
  tone: "blue" | "indigo" | "cyan" | "emerald" | "sky" | "violet" | "orange" | "purple" | "pink" | "teal";
};

const toneClasses: Record<FeatureDetail["tone"], { icon: string; iconHover: string }> = {
  blue: { icon: "bg-blue-50 text-blue-600", iconHover: "group-hover:bg-blue-100 group-focus-visible:bg-blue-100" },
  indigo: { icon: "bg-indigo-50 text-indigo-600", iconHover: "group-hover:bg-indigo-100 group-focus-visible:bg-indigo-100" },
  cyan: { icon: "bg-cyan-50 text-cyan-600", iconHover: "group-hover:bg-cyan-100 group-focus-visible:bg-cyan-100" },
  emerald: { icon: "bg-emerald-50 text-emerald-600", iconHover: "group-hover:bg-emerald-100 group-focus-visible:bg-emerald-100" },
  sky: { icon: "bg-sky-50 text-sky-600", iconHover: "group-hover:bg-sky-100 group-focus-visible:bg-sky-100" },
  violet: { icon: "bg-violet-50 text-violet-600", iconHover: "group-hover:bg-violet-100 group-focus-visible:bg-violet-100" },
  orange: { icon: "bg-orange-50 text-orange-600", iconHover: "group-hover:bg-orange-100 group-focus-visible:bg-orange-100" },
  purple: { icon: "bg-purple-50 text-purple-600", iconHover: "group-hover:bg-purple-100 group-focus-visible:bg-purple-100" },
  pink: { icon: "bg-pink-50 text-pink-600", iconHover: "group-hover:bg-pink-100 group-focus-visible:bg-pink-100" },
  teal: { icon: "bg-teal-50 text-teal-600", iconHover: "group-hover:bg-teal-100 group-focus-visible:bg-teal-100" },
};

const featureAnchorIds: Record<string, string[]> = {
  "create-card": ["digital-cards"],
  "customize-design": ["templates", "branding"],
  "instant-contact": ["connect"],
  "qr-sharing": ["qr"],
  "nfc-sharing": ["nfc"],
  "save-contact": ["save-contact"],
  "email-sharing": ["email-sharing"],
  "manage-contacts": ["contacts"],
  "analytics-visual": ["analytics"],
  "team-cards": ["team"],
};

export const featureDetails: FeatureDetail[] = [
  {
    id: "profile",
    title: "Profile photo and personal details",
    description: "Add your profile photo, full name, designation and personal information so visitors immediately know who you are.",
    howItWorks: "Your personal details appear at the top of your digital business card.",
    examples: ["Praveen Kumar", "Founder", "Profile photo"],
    icon: UserRound,
    cardDescription: "Make your first impression personal.", tone: "blue",
  },
  {
    id: "company",
    title: "Job title and company information",
    description: "Show your professional role, company name and business information.",
    howItWorks: "Visitors can quickly understand your position and the organisation you represent.",
    examples: ["Managing Director", "ABC Technologies Pvt Ltd"],
    icon: Building2,
    cardDescription: "Show visitors who you represent.", tone: "indigo",
  },
  {
    id: "phone",
    title: "Phone number and click-to-call",
    description: "Allow visitors to call you directly from your digital business card.",
    howItWorks: "When a visitor taps the Call button, the device opens the phone dialer with your configured number.",
    examples: ["Tap Call → phone dialer opens."],
    icon: Phone,
    cardDescription: "Let visitors call in one tap.", tone: "cyan",
  },
  {
    id: "whatsapp",
    title: "WhatsApp direct contact",
    description: "Let visitors start a WhatsApp conversation with you instantly.",
    howItWorks: "The WhatsApp button opens a chat with your configured WhatsApp number.",
    examples: ["Tap WhatsApp → WhatsApp conversation opens."],
    icon: MessageCircle,
    cardDescription: "Start a direct WhatsApp chat.", tone: "emerald",
  },
  {
    id: "email",
    title: "Email address",
    description: "Allow visitors to contact you quickly by email.",
    howItWorks: "When the Email button is clicked, the visitor's email application opens with your email address already selected.",
    examples: ["Tap Email → compose email."],
    icon: Mail,
    cardDescription: "Open a ready-to-address email.", tone: "sky",
  },
  {
    id: "website",
    title: "Website and custom links",
    description: "Add your website, portfolio, product pages or any important custom links.",
    howItWorks: "Visitors can tap a link and open the configured website in their browser.",
    examples: ["Company Website", "Portfolio", "Book Appointment", "Product Catalogue"],
    icon: Globe2,
    cardDescription: "Share your most important destinations.", tone: "violet",
  },
  {
    id: "map",
    title: "Address and map location",
    description: "Share your office, shop or business location.",
    howItWorks: "When the visitor taps the location button, the configured map/location opens.",
    examples: ["Tap Directions → map application opens."],
    icon: MapPin,
    cardDescription: "Help visitors find your business.", tone: "orange",
  },
  {
    id: "qr",
    title: "QR code sharing",
    description: "Share your digital business card using a QR code.",
    howItWorks: "A QR code points to your public digital card. Anyone can scan it using a phone camera.",
    examples: ["Display the QR on a business card, poster, shop counter or event stand."],
    icon: QrCode,
    cardDescription: "Make your card easy to scan and share.", tone: "blue",
  },
  {
    id: "nfc",
    title: "NFC sharing",
    description: "Share your digital business card with a simple NFC tap.",
    howItWorks: "An NFC tag/card is linked to your public digital card URL. A compatible phone can open the card when tapped.",
    examples: ["Tap an NFC card to a compatible phone → digital business card opens."],
    icon: Nfc,
    cardDescription: "Share your card with a compatible tap.", tone: "purple",
  },
  {
    id: "social",
    title: "Social media profiles",
    description: "Connect visitors to your social media profiles from one digital card.",
    howItWorks: "Add supported social profile URLs and visitors can open them directly.",
    examples: ["LinkedIn", "Instagram", "Facebook", "YouTube", "X/Twitter"],
    icon: Share2,
    cardDescription: "Bring your profiles together.", tone: "pink",
  },
  {
    id: "contact",
    title: "Save Contact",
    description: "Allow visitors to save your contact information directly to their phone.",
    howItWorks: "The Save Contact action downloads or opens a vCard/contact file containing your configured information.",
    examples: ["Tap Save Contact → Add Contact screen opens."],
    icon: ContactRound,
    cardDescription: "Let visitors save your details.", tone: "teal",
  },
  {
    id: "analytics",
    title: "Views and engagement analytics",
    description: "Understand how people interact with your digital business card.",
    howItWorks: "The product records supported card views, QR scans, NFC taps, contact exchanges and link clicks for your cards.",
    examples: ["View the recorded totals for card views, QR scans, NFC taps and link clicks."],
    icon: BarChart3,
    cardDescription: "Learn how visitors engage.", tone: "indigo",
  },
];

type FeatureDetailsModalProps = {
  feature: FeatureDetail;
  onClose: () => void;
  openerRef: React.RefObject<HTMLButtonElement | null>;
};

export function FeatureDetailsModal({ feature, onClose, openerRef }: FeatureDetailsModalProps) {
  const { t } = useCustomerLocale();
  const dialogRef = useRef<HTMLElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const Icon = feature.icon;
  const tone = toneClasses[feature.tone];

  useEffect(() => {
    const priorOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeRef.current?.focus();

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
        return;
      }
      if (event.key !== "Tab" || !dialogRef.current) return;
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])',
      ));
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = priorOverflow;
      document.removeEventListener("keydown", onKeyDown);
      openerRef.current?.focus();
    };
  }, [onClose, openerRef]);

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-slate-950/50 p-4 backdrop-blur-sm animate-[feature-backdrop-in_180ms_ease-out_both] motion-reduce:animate-none sm:p-6"
      onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}
    >
      <section
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="feature-details-title"
        aria-describedby="feature-details-description"
        tabIndex={-1}
        className={`relative max-h-[calc(100dvh-32px)] w-full ${feature.illustration ? "max-w-[900px]" : "max-w-[650px]"} overflow-y-auto overscroll-contain rounded-[28px] bg-white p-5 shadow-[0_32px_100px_-24px_rgba(15,23,42,0.55)] animate-[feature-dialog-in_200ms_ease-out_both] motion-reduce:animate-none sm:p-8`}
      >
        <button
          ref={closeRef}
          type="button"
          onClick={onClose}
          aria-label={t("Close feature details")}
          className="absolute right-3 top-3 z-10 rounded-xl bg-white p-3 text-slate-500 shadow-sm transition hover:bg-slate-100 hover:text-slate-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"
        >
          <X aria-hidden="true" className="h-5 w-5" />
        </button>

        <div className={feature.illustration ? "grid items-start gap-6 md:grid-cols-[0.85fr_1.15fr]" : ""}>
        {feature.illustration && <div className="md:sticky md:top-0"><FeatureIllustration src={feature.illustration} alt={feature.illustrationAlt || feature.title} /><p className="mt-3 text-center text-xs font-medium text-slate-500">{feature.id === "analytics-visual" ? t("Illustrative example · not live data") : t("A closer look at how it works")}</p></div>}
        <div className="min-w-0">
        {!feature.illustration && <div className={`flex h-14 w-14 items-center justify-center rounded-2xl ${tone.icon} shadow-sm ring-1 ring-inset ring-black/5`}>
          <Icon aria-hidden="true" className="h-7 w-7" />
        </div>}
        <h2 id="feature-details-title" className="mt-5 pr-8 text-2xl font-black text-slate-950 sm:text-3xl">
          {t(feature.heading || feature.title)}
        </h2>
        <p id="feature-details-description" className="mt-3 leading-7 text-slate-600">
          {t(feature.description)}
        </p>

        <div className="mt-6 rounded-2xl bg-slate-50 p-5">
          <h3 className="font-bold text-slate-900">{t("How it works")}</h3>
          <p className="mt-2 leading-7 text-slate-600">{t(feature.howItWorks)}</p>
          {feature.steps && <ol className="mt-4 space-y-3">{feature.steps.map((step, index) => <li key={step} className="flex items-start gap-3 text-sm leading-6 text-slate-700"><span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-blue-100 text-xs font-bold text-blue-700">{index + 1}</span><span>{t(step)}</span></li>)}</ol>}
        </div>

        <div className="mt-4 rounded-2xl border border-blue-100 bg-blue-50/60 p-5">
          <h3 className="font-bold text-slate-900">{t("Example workflow")}</h3>
          <ul className="mt-2 space-y-1.5 text-slate-700">
            {feature.examples.map((example) => <li key={example}>{t(example)}</li>)}
          </ul>
        </div>
        {feature.benefits && <ul aria-label={t("Feature highlights")} className="mt-4 flex flex-wrap gap-2">{feature.benefits.map(benefit => <li key={benefit} className="rounded-lg bg-cyan-50 px-3 py-1.5 text-xs font-semibold text-cyan-900">{t(benefit)}</li>)}</ul>}

        <div className="mt-7 flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <button type="button" onClick={onClose} className="rounded-xl border border-slate-300 px-5 py-3 font-semibold text-slate-700 transition hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600">
            {t("Close")}
          </button>
          <Link href={feature.ctaHref || "/register"} className="rounded-xl bg-blue-600 px-5 py-3 text-center font-semibold text-white transition hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2">
            {t(feature.ctaLabel || "Create Your Card")}
          </Link>
        </div>
        </div>
        </div>
      </section>
    </div>
  );
}

type FeatureDetailsGridProps = {
  featureIds?: string[];
  gridClassName?: string;
  premium?: boolean;
  illustrated?: boolean;
};

export default function FeatureDetailsGrid({
  featureIds,
  gridClassName = "grid gap-5 sm:grid-cols-2 lg:grid-cols-3",
  premium = false,
  illustrated = false,
}: FeatureDetailsGridProps) {
  const { t } = useCustomerLocale();
  const [selected, setSelected] = useState<FeatureDetail | null>(null);
  const openerRef = useRef<HTMLButtonElement>(null);
  const source = illustrated ? visualFeatures : featureDetails;
  const visibleFeatures = (featureIds || source.map(feature => feature.id)).map((id) => source.find((feature) => feature.id === id)).filter((feature): feature is FeatureDetail => Boolean(feature));
  const closeModal = () => setSelected(null);

  return (
    <>
      <div className={gridClassName}>
        {visibleFeatures.map((feature) => {
          const Icon = feature.icon;
          if (illustrated) return <button key={feature.id} type="button" onClick={event => { openerRef.current = event.currentTarget; setSelected(feature); }} aria-haspopup="dialog" className="group relative flex min-w-0 cursor-pointer flex-col rounded-[24px] border border-slate-200 bg-white p-3 text-left shadow-sm transition duration-300 hover:-translate-y-1 hover:border-blue-200 hover:shadow-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-4 motion-reduce:transform-none motion-reduce:transition-none">
            {featureAnchorIds[feature.id]?.map(anchor => <span key={anchor} id={anchor} aria-hidden="true" className="absolute left-0 top-[-6rem]" />)}
            <FeatureIllustration src={feature.illustration!} alt={feature.illustrationAlt || feature.title} />
            <span className="flex flex-1 flex-col px-2 pb-2 pt-5"><span className="text-lg font-bold leading-6 text-slate-900">{t(feature.title)}</span><span className="mt-2 text-sm leading-6 text-slate-600">{t(feature.shortDescription || "")}</span><span className="mt-auto flex items-center justify-between border-t border-blue-100 pt-3 text-xs font-bold text-blue-700"><span className="mt-3">{t("See how it works")}</span><span aria-hidden="true" className="mt-3 text-xl transition-transform group-hover:translate-x-1 motion-reduce:transform-none">→</span></span></span>
          </button>;
          return (
            <button
              key={feature.id}
              type="button"
              onClick={(event) => { openerRef.current = event.currentTarget; setSelected(feature); }}
              aria-haspopup="dialog"
              className={premium
                ? "group relative isolate flex min-h-[154px] w-full cursor-pointer flex-col items-start overflow-hidden rounded-[20px] border border-slate-200/80 bg-white p-5 text-left shadow-[0_8px_28px_-18px_rgba(15,23,42,0.24)] transition-all duration-300 hover:-translate-y-1 hover:border-blue-200 hover:shadow-[0_22px_44px_-24px_rgba(37,99,235,0.36)] motion-reduce:transform-none motion-reduce:transition-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2 sm:p-6"
                : "group flex w-full cursor-pointer items-center gap-4 rounded-2xl border border-slate-200 bg-white p-5 text-left shadow-sm transition-all duration-300 hover:-translate-y-1 hover:border-blue-300 hover:shadow-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2"}
            >
              {premium && <span aria-hidden="true" className="pointer-events-none absolute inset-0 z-0 bg-gradient-to-br from-blue-50/70 via-white to-indigo-50/50 opacity-0 transition-opacity duration-300 group-hover:opacity-100 group-focus-visible:opacity-100" />}
              <span className={`relative z-10 flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl transition-colors duration-200 ${premium ? `${toneClasses[feature.tone].icon} ${toneClasses[feature.tone].iconHover}` : "bg-blue-50 text-blue-600 group-hover:bg-blue-600 group-hover:text-white group-focus-visible:bg-blue-600 group-focus-visible:text-white"}`}>
                <Icon aria-hidden="true" className="h-7 w-7" />
              </span>
              <span className={premium ? "relative z-10 mt-4 flex w-full items-center gap-2" : "flex w-full items-center gap-2"}>
                <span className="font-bold text-slate-900 transition-colors group-hover:text-blue-700">{t(feature.title)}</span>
                <span aria-hidden="true" className="ml-auto shrink-0 text-blue-600 transition-transform duration-200 group-hover:translate-x-1">→</span>
              </span>
              {premium && <span className="relative z-10 mt-1 text-sm text-slate-500">{t(feature.cardDescription)}</span>}
            </button>
          );
        })}
      </div>
      {selected && <FeatureDetailsModal feature={selected} onClose={closeModal} openerRef={openerRef} />}
    </>
  );
}

