"use client";
import { useEffect, useState } from "react";
import { apiRequest } from "@/lib/user-api";
export type DesignSettings = { theme:string; primary_color:string; secondary_color:string; background_color:string; text_color:string; accent_color:string; font_family:string; font_scale:number; layout:string; button_style:string; border_radius:string; profile_style:string; logo_position:string; social_icon_style:string; section_order:string[] };
export type Card = { id: number; public_id: string; name: string; slug: string; template_id: number; template_name: string; template_preview_image_url?: string; company_name: string; job_title: string; email: string; phone: string; website: string; address: string; photo_url: string; logo_url: string; primary_color: string; design_settings?:Partial<DesignSettings>; status: string; published_at?: string | null; views: number; qr_count: number; nfc_count: number; qr_scans: number; nfc_taps: number; updated_at: string; social_links: {label: string; url: string}[]; custom_links: {label: string; url: string}[]; videos: {label: string; url: string}[] };
export type CardList = { data: Card[]; pagination: { page: number; pages: number; total: number } };
export type Summary = { total_cards: number; published_cards: number; draft_cards: number; total_qr_scans: number; total_nfc_taps: number; total_contacts: number; current_plan: { plan_id: number; name: string; status: string; amount: number; currency: string; billing_period: string; started_at: string; renews_at: string | null; ends_at: string | null; features: string[] | null; limits: Record<string, unknown> | null } | null };
export type Template = { id: number; name: string; description: string; preview_image_url: string; category: string; design_settings?:Partial<DesignSettings> };
export function useUserData<T>(path: string) {
  const [state, setState] = useState<{ data: T | null; error: string; loading: boolean }>({ data: null, error: "", loading: true });
  const [version, setVersion] = useState(0);
  useEffect(() => {
    let live = true; setState({ data: null, error: "", loading: true });
    apiRequest(path).then(data => { if (live) setState({ data, error: "", loading: false }); }).catch(e => { if (live) setState({ data: null, error: e.message, loading: false }); });
    return () => { live = false; };
  }, [path, version]);
  return { ...state, reload: () => setVersion(v => v + 1) };
}
export const dateLabel = (value: string | null) => value ? new Date(value).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }) : "—";
