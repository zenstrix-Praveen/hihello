"use client";
import { getApiBaseUrl } from "@/lib/api-config";
import { useCustomerLocale } from "@/lib/customer-locale";
import { useState } from "react";
import { apiRequest } from "@/lib/user-api";
import { CardList, useUserData } from "@/components/dashboard/data";
import { LoadState } from "@/components/dashboard/Widgets";

type QR = {id:number;code_value:string;status:string;card_name:string;card_slug:string;scan_count:number;created_at:string};
const API = getApiBaseUrl();
const actionClass="rounded-lg border border-slate-200 px-3 py-2 text-sm font-semibold text-blue-700 transition hover:border-blue-300 hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 disabled:opacity-50";
export default function QRCodesPage(){
 const { t, error: localizeError, language } = useCustomerLocale();
 

 const cards=useUserData<CardList>("/user/cards?limit=100"); const qrs=useUserData<{data:QR[]}> ("/user/qr-codes"); const [busy,setBusy]=useState(0); const [error,setError]=useState(""); const [success,setSuccess]=useState("");
 async function create(cardId:number){setBusy(cardId);setError("");setSuccess("");try{await apiRequest(`/user/cards/${cardId}/qr`,{method:"POST"});qrs.reload();cards.reload();setSuccess("QR code is ready for this published card.");}catch(e){setError(e instanceof Error?e.message:"Could not create QR code");}finally{setBusy(0);}}
 async function downloadQR(slug:string){setError("");try{const response=await fetch(`${API}/public/cards/${encodeURIComponent(slug)}/qr`);if(!response.ok)throw new Error("QR image is unavailable.");const objectURL=URL.createObjectURL(await response.blob());const link=document.createElement("a");link.href=objectURL;link.download=`${slug}-qr.png`;document.body.appendChild(link);link.click();link.remove();window.setTimeout(()=>URL.revokeObjectURL(objectURL),1000);}catch(e){setError(e instanceof Error?e.message:"Could not download QR image");}}
 async function copyCardLink(slug:string):Promise<boolean>{try{await navigator.clipboard.writeText(`${window.location.origin}/c/${encodeURIComponent(slug)}`);setSuccess("Public card link copied.");setError("");return true;}catch{setError("Could not access the clipboard. Open the card page to copy its link.");setSuccess("");return false;}}
 async function shareQR(qr:QR){
  const url=`${window.location.origin}/c/${encodeURIComponent(qr.card_slug)}`;
  const title=t("{name} - Digital Business Card",{name:qr.card_name});
  const shareURL=async()=>{if(navigator.share){await navigator.share({title,text:t("View my digital business card"),url});return true;}return false;};
  setError("");setSuccess("");
  try{
   if(navigator.share){
    try{
     const response=await fetch(`${API}/public/cards/${encodeURIComponent(qr.card_slug)}/qr`);
     if(!response.ok)throw new Error("QR image unavailable");
     const blob=await response.blob();
     if(blob.type!=="image/png"||!blob.size)throw new Error("QR image unavailable");
     const file=new File([blob],`dbizcard-${qr.card_slug}-qr.png`,{type:"image/png"});
     if(navigator.canShare?.({files:[file]})){
      await navigator.share({title,text:t("View my digital business card"),url,files:[file]});
      return;
     }
    }catch(error){if(error instanceof DOMException&&error.name==="AbortError")return;}
    try{if(await shareURL())return;}catch(error){if(error instanceof DOMException&&error.name==="AbortError")return;}
   }
   if(!await copyCardLink(qr.card_slug))throw new Error("Clipboard unavailable");
   setSuccess("Card link copied. You can share it anywhere.");
  }catch{
   setError("Unable to share QR code. You can download it or copy the card link instead.");
   setSuccess("");
  }
 }
 const published=cards.data?.data.filter(card=>card.status==="published")||[];
 return <div className="space-y-6"><div><h1 className="text-3xl font-black">{t("QR Codes")}</h1><p className="mt-2 text-slate-600">{t("Generate and view QR codes for your published digital cards.")}</p></div>{error&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{localizeError(error)}</p>}{success&&<p role="status" className="rounded-xl bg-emerald-50 p-3 text-sm text-emerald-700">{t(success)}</p>}<section className="rounded-2xl border border-slate-200 bg-white p-6"><h2 className="text-lg font-bold">{t("Published Cards")}</h2><LoadState {...cards} retry={cards.reload}/>{cards.data&&(published.length?<div className="mt-4 divide-y">{published.map(card=><div key={card.id} className="flex flex-wrap items-center justify-between gap-4 py-4"><div><p className="font-semibold">{card.name}</p><p className="text-sm text-slate-500"><bdi dir="ltr">/{card.slug}</bdi></p></div><button disabled={busy===card.id} onClick={()=>create(card.id)} className="rounded-xl bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60">{busy===card.id?t("Preparing…"):t("Generate / View QR")}</button></div>)}</div>:<p className="mt-4 text-sm text-slate-500">{t("Publish a card before generating its QR code.")}</p>)}</section><section className="overflow-hidden rounded-2xl border border-slate-200 bg-white"><h2 className="p-6 text-lg font-bold">{t("Your QR Codes")}</h2><LoadState {...qrs} retry={qrs.reload}/>{qrs.data&&(qrs.data.data.length?<div className="grid gap-4 p-5 sm:grid-cols-2 lg:grid-cols-3">{qrs.data.data.map(q=><article key={q.id} className="rounded-xl border border-slate-200 p-5"><div className="flex items-start justify-between gap-3"><div><h3 className="font-semibold">{q.card_name}</h3><p className="mt-1 text-sm text-slate-500">{q.scan_count.toLocaleString(language === "ar" ? "ar" : "en")} {t("scans")}</p></div><span className={`rounded-full px-2.5 py-1 text-xs font-semibold capitalize ${q.status==="active"?"bg-emerald-50 text-emerald-700":"bg-slate-100 text-slate-600"}`}>{t(q.status)}</span></div><img src={`${API}/public/cards/${encodeURIComponent(q.card_slug)}/qr`} alt={t("QR code for {name}",{name:q.card_name})} className="mx-auto mt-4 h-48 w-48 rounded-lg border bg-white p-2"/><div className="mt-4 flex flex-wrap justify-center gap-2"><button type="button" onClick={()=>downloadQR(q.card_slug)} className={actionClass}>{t("Download QR")}</button><button type="button" onClick={()=>copyCardLink(q.card_slug)} className={actionClass}>{t("Copy card link")}</button>{q.status==="active"&&<button type="button" aria-label={t("Share QR for {name}",{name:q.card_name})} onClick={()=>void shareQR(q)} className={actionClass}>{t("Share QR")}</button>}</div></article>)}</div>:<p className="px-6 pb-8 text-slate-500">{t("No QR codes yet.")}</p>)}</section></div>;
}
