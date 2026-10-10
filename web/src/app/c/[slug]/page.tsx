import { getApiBaseUrl } from "@/lib/api-config";
import type { Metadata } from "next";
import PublicCard from "@/components/cards/PublicCard";
import PublicCardUnavailable from "@/components/cards/PublicCardUnavailable";

const API = getApiBaseUrl();
const SERVER_API = process.env.API_SERVER_URL || API;
type PublicData = { id: number; public_id?: string; name: string; slug: string; job_title: string; company_name: string; phone: string; email: string; website: string; address: string; photo_url: string; logo_url: string; primary_color: string; template_id?: number; template_name?: string; template_preview_image_url?: string; published_at?: string; social_links: { label: string; url: string }[]; custom_links: { label: string; url: string }[]; videos: { label: string; url: string }[] };
async function loadCard(slug: string, metadata = false): Promise<PublicData | null> {
  try {
    const response = await fetch(`${SERVER_API}/public/cards/${encodeURIComponent(slug)}${metadata ? "?metadata=1" : ""}`, { cache: "no-store" });
    if (!response.ok) return null;
    const result = await response.json();
    return result?.data || null;
  } catch { return null; }
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const card = await loadCard(slug, true);
  if (!card) return { title: "Card not available | Digital Business Card", robots: { index: false, follow: false } };
  const summary = [card.job_title, card.company_name].filter(Boolean).join(" at ");
  return { title: `${card.name} | Digital Business Card`, description: summary || "Professional digital business card", robots: { index: true, follow: true } };
}

export default async function PublicCardPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const card = await loadCard(slug);
  if (!card) return <PublicCardUnavailable />;
  return <PublicCard card={card} apiBase={API} />;
}
