"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import { Card, CardList, useUserData } from "./data";
import { LoadState } from "./Widgets";
import CardTable from "./CardTable";
import MetricCard from "./MetricCard";
import { ContactRound, Eye, MousePointerClick, QrCode, SmartphoneNfc } from "lucide-react";
type EventTotals = {card_views:number;qr_scans:number;nfc_taps:number;contact_exchanges:number;link_clicks:number};
export default function Analytics({cardID}:{cardID?:string}) {
 const { t } = useCustomerLocale();
 

 const analytics=useUserData<EventTotals>(`/user/analytics${cardID?`?card_id=${encodeURIComponent(cardID)}`:""}`);
 const metrics: [string,keyof EventTotals,typeof Eye][] = [["Card Views","card_views",Eye],["QR Scans","qr_scans",QrCode],["NFC Taps","nfc_taps",SmartphoneNfc],["Contact Exchanges","contact_exchanges",ContactRound],["Link Clicks","link_clicks",MousePointerClick]];
 return <div className="space-y-6"><h1 className="text-3xl font-bold">{t("Analytics")}</h1><p className="text-sm text-slate-500">{t("Activity recorded for your cards only.")}</p><LoadState {...analytics} retry={analytics.reload}/>{analytics.data&&<div className="grid grid-cols-2 gap-4 xl:grid-cols-3">{metrics.map(([label,key,Icon])=><MetricCard key={key} label={t(label)} value={analytics.data?.[key] ?? 0} Icon={Icon}/>)}</div>}<CardAnalytics cardID={cardID}/></div>;
}
function CardAnalytics({cardID}:{cardID?:string}) {
 const { t } = useCustomerLocale();
 

 const state=useUserData<CardList | {data:Card}>(cardID ? `/user/cards/${encodeURIComponent(cardID)}` : "/user/cards?limit=100");
 const cards=state.data ? (Array.isArray(state.data.data) ? state.data.data : [state.data.data]) : [];
 return <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white"><h2 className="p-6 text-lg font-bold">{cardID ? t("Card activity") : t("Recently updated cards (up to 100)")}</h2><LoadState {...state} retry={state.reload}/>{state.data && <><CardTable cards={cards} compact/><div className="divide-y">{cards.map(c=><div key={c.id} className="flex flex-wrap justify-between gap-3 p-5 text-sm"><span className="font-semibold">{c.name}</span><span className="text-slate-500">{c.views} {t("views ·")} {c.qr_scans} {t("QR scans ·")} {c.nfc_taps} {t("NFC taps")}</span></div>)}</div></>}</section>;
}
