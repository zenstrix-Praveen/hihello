"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import Image from "next/image";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { ArrowRight, Check, CircleHelp, Copy, ExternalLink, Info, Nfc, Pencil, Plus, Radio, Smartphone, Wifi, X } from "lucide-react";
import { apiRequest } from "@/lib/user-api";
import { CardList, useUserData } from "@/components/dashboard/data";
import { LoadState } from "@/components/dashboard/Widgets";

type NFCItem = { id:number; public_id:string; name:string; tag_hint:string; card_id:number; card_name:string; card_slug:string; card_status:string; template_name:string; status:"active"|"disabled"|"archived"; tap_count:number; last_tapped_at:string|null; created_at:string; updated_at:string; redirect_url:string };
type NFCList = { data:NFCItem[]; limit:number; used:number };
type FormValue = { name:string; tag_uid:string; card_id:string; status:"active"|"disabled" };
const blankForm = ():FormValue => ({name:"",tag_uid:"",card_id:"",status:"active"});
const actionClass="inline-flex min-h-10 items-center justify-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-sm font-semibold text-blue-700 transition hover:border-blue-300 hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50";
const inputClass="mt-1.5 block min-h-11 w-full rounded-xl border border-slate-300 bg-white px-3.5 py-2.5 text-sm text-slate-900 outline-none transition placeholder:text-slate-400 focus:border-blue-500 focus:ring-2 focus:ring-blue-100";

function Dialog({ title, onClose, children, wide=false }: { title:string; onClose:()=>void; children:React.ReactNode; wide?:boolean }) {
 const { t } = useCustomerLocale();

  const dialogRef=useRef<HTMLElement>(null);
  const closeRef=useRef<HTMLButtonElement>(null);
  const closeHandler=useRef(onClose);
  closeHandler.current=onClose;
  useEffect(()=>{
    const previousOverflow=document.body.style.overflow;
    const previousFocus=document.activeElement instanceof HTMLElement?document.activeElement:null;
    document.body.style.overflow="hidden";
    closeRef.current?.focus();
    const onKeyDown=(event:KeyboardEvent)=>{
      if(event.key==="Escape"){event.preventDefault();closeHandler.current();return;}
      if(event.key!=="Tab"||!dialogRef.current)return;
      const focusable=[...dialogRef.current.querySelectorAll<HTMLElement>('button:not([disabled]),a[href],input:not([disabled]),select:not([disabled]),textarea:not([disabled])')];
      if(!focusable.length)return;
      if(event.shiftKey&&document.activeElement===focusable[0]){event.preventDefault();focusable[focusable.length-1].focus();}
      else if(!event.shiftKey&&document.activeElement===focusable[focusable.length-1]){event.preventDefault();focusable[0].focus();}
    };
    window.addEventListener("keydown",onKeyDown);
    return ()=>{document.body.style.overflow=previousOverflow;window.removeEventListener("keydown",onKeyDown);previousFocus?.focus();};
  },[]);
  return <div className="fixed inset-0 z-[100] flex items-end justify-center bg-slate-950/50 p-0 backdrop-blur-sm sm:items-center sm:p-4" onMouseDown={event=>{if(event.target===event.currentTarget)onClose();}}>
    <section ref={dialogRef} role="dialog" aria-modal="true" aria-labelledby="nfc-dialog-title" className={`max-h-[96dvh] w-full ${wide?"max-w-3xl":"max-w-xl"} overflow-y-auto rounded-t-[24px] bg-white p-5 shadow-2xl sm:max-h-[92dvh] sm:rounded-[24px] sm:p-7`}>
      <div className="flex items-start justify-between gap-4"><div><p className="text-xs font-bold uppercase tracking-[.17em] text-blue-700">{t("Dbizcard NFC")}</p><h2 id="nfc-dialog-title" className="mt-1 text-xl font-black text-slate-950 sm:text-2xl">{t(title)}</h2></div><button ref={closeRef} type="button" onClick={onClose} aria-label={t("Close dialog")} className="rounded-xl p-2 text-slate-500 hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"><X aria-hidden="true" className="h-5 w-5"/></button></div>
      {children}
    </section>
  </div>;
}

export default function NFCPage() {
 const { t, error: localizeError, language } = useCustomerLocale();

  const cards = useUserData<CardList>("/user/cards?limit=100");
  const tags = useUserData<NFCList>("/user/nfc");
  const [mode,setMode]=useState<"create"|"edit"|null>(null);
  const [editing,setEditing]=useState<NFCItem|null>(null);
  const [created,setCreated]=useState<NFCItem|null>(null);
  const [showHow,setShowHow]=useState(false);
  const [step,setStep]=useState(1);
  const [form,setForm]=useState<FormValue>(blankForm());
  const [saving,setSaving]=useState(false);
  const [error,setError]=useState("");
  const [notice,setNotice]=useState("");
  const publishedCards=cards.data?.data.filter(card=>card.status==="published")||[];
  const isLimited=tags.data ? tags.data.limit===0 || (tags.data.limit>0 && tags.data.used>=tags.data.limit) : false;

  function startCreate(){setMode("create");setEditing(null);setCreated(null);setForm(blankForm());setStep(1);setError("");setNotice("");}
  function startEdit(tag:NFCItem){setEditing(tag);setCreated(null);setMode("edit");setForm({name:tag.name,tag_uid:"",card_id:String(tag.card_id),status:tag.status==="active"?"active":"disabled"});setStep(2);setError("");setNotice("");}
  function closeDialog(){setMode(null);setEditing(null);setCreated(null);setError("");}

  async function copyLink(url:string){
    try { await navigator.clipboard.writeText(url); setNotice("NFC link copied."); setError(""); }
    catch { setError("Could not access the clipboard. Select and copy the NFC URL manually."); setNotice(""); }
  }
  function testLink(url:string){const testURL=new URL(url);testURL.searchParams.set("test","1");window.open(testURL.toString(),"_blank","noopener,noreferrer");}

  async function saveTag(event:React.FormEvent<HTMLFormElement>){
    event.preventDefault();setSaving(true);setError("");setNotice("");
    try {
      const payload={name:form.name.trim(),tag_uid:form.tag_uid.trim(),card_id:Number(form.card_id),status:form.status};
      const response=await apiRequest(editing?`/user/nfc/${editing.id}`:"/user/nfc",{method:editing?"PUT":"POST",body:JSON.stringify(payload)}) as {data:NFCItem};
      if(editing){setNotice("NFC tag updated.");setMode(null);setEditing(null);tags.reload();}
      else {setCreated(response.data);tags.reload();setMode(null);}
    } catch(err) { setError(err instanceof Error?err.message:"Could not save NFC tag"); }
    finally { setSaving(false); }
  }

  async function toggle(tag:NFCItem){
    const next=tag.status==="active"?"disabled":"active";
    setError("");setNotice("");
    try { await apiRequest(`/user/nfc/${tag.id}`,{method:"PUT",body:JSON.stringify({name:tag.name,card_id:tag.card_id,status:next})});tags.reload();setNotice(next==="active"?"NFC tag enabled.":"NFC tag disabled."); }
    catch(err){setError(err instanceof Error?err.message:"Could not update NFC tag");}
  }

  async function archive(tag:NFCItem){
    if(!window.confirm(t("Archive “{name}”? Its tap history will be kept, and the NFC link will stop working.",{name:tag.name})))return;
    setError("");setNotice("");
    try { await apiRequest(`/user/nfc/${tag.id}`,{method:"DELETE"});tags.reload();setNotice("NFC tag archived. Tap history is retained."); }
    catch(err){setError(err instanceof Error?err.message:"Could not archive NFC tag");}
  }

  const tagAllowance=tags.data?.limit===-1?t("Unlimited"):tags.data?`${tags.data.used} / ${tags.data.limit}`:"—";

  return <div className="mx-auto max-w-6xl space-y-6 sm:space-y-8">
    <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between"><div><p className="text-xs font-extrabold uppercase tracking-[.18em] text-blue-700">{t("Tap to connect")}</p><h1 className="mt-2 text-3xl font-black tracking-tight text-slate-950 sm:text-4xl">{t("NFC Tags")}</h1><p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600 sm:text-base">{t("Connect an NFC card or tag to your digital business card and share it with a simple tap.")}</p></div><div className="flex flex-col gap-2 sm:flex-row"><button type="button" onClick={()=>setShowHow(true)} className={actionClass}><CircleHelp aria-hidden="true" className="h-4 w-4"/>{t("How NFC Works")}</button><button type="button" onClick={startCreate} disabled={publishedCards.length===0||isLimited} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-600 px-4 py-2.5 font-bold text-white shadow-sm hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"><Plus aria-hidden="true" className="h-5 w-5"/>{t("Add NFC Tag")}</button></div></div>
    {error&&<p role="alert" className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800">{localizeError(error)}{/NFC tag limit/i.test(error)&&<Link className="ms-2 inline-flex font-bold text-blue-700 underline" href="/dashboard/billing">{t("Upgrade Plan")}</Link>}</p>}
    {notice&&<p role="status" className="rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm font-medium text-emerald-800">{t(notice)}</p>}

    <LoadState {...tags} retry={tags.reload}/>
    {tags.data&&<>
      <div className="grid gap-3 sm:grid-cols-3"><Metric label="Total Tags" value={String(tags.data.data.length)} icon={Nfc}/><Metric label="Active Tags" value={String(tags.data.data.filter(tag=>tag.status==="active").length)} icon={Radio}/><Metric label="Recorded Taps" value={tags.data.data.reduce((sum,tag)=>sum+tag.tap_count,0).toLocaleString(language === "ar" ? "ar" : "en")} icon={Wifi}/></div>
      <div className="flex flex-wrap items-center justify-between gap-2 rounded-2xl border border-blue-100 bg-blue-50/70 px-4 py-3 text-sm"><span className="font-semibold text-slate-800">{t("NFC tag allowance")}</span><span className="font-bold text-blue-800">{tagAllowance}</span>{tags.data.limit===0&&<Link href="/dashboard/billing" className="font-bold text-blue-700 underline">{t("Upgrade Plan")}</Link>}</div>
      {tags.data.data.length===0?<section className="overflow-hidden rounded-[28px] border border-slate-200 bg-white shadow-sm"><div className="grid items-center gap-7 p-5 sm:p-8 lg:grid-cols-[.9fr_1.1fr] lg:p-10"><div className="relative mx-auto w-full max-w-md"><Image src="/features/nfc-sharing.webp" alt={t("Illustration of phones and an NFC tag sharing a digital card.")} width={800} height={600} sizes="(max-width: 1024px) 90vw, 440px" className="h-auto w-full rounded-[24px]"/></div><div><span className="inline-flex rounded-2xl bg-cyan-50 p-3 text-cyan-700"><Nfc aria-hidden="true" className="h-7 w-7"/></span><h2 className="mt-4 text-2xl font-black text-slate-950">{t("No NFC tags yet")}</h2><p className="mt-3 max-w-lg leading-7 text-slate-600">{t("Add an NFC tag and connect it to one of your published digital cards. Dbizcard will provide a link to configure with compatible NFC hardware.")}</p>{publishedCards.length===0&&<p className="mt-3 text-sm font-medium text-amber-800">{t("Publish a digital card before adding an NFC tag.")}</p>}<div className="mt-6 flex flex-col gap-3 sm:flex-row"><button type="button" onClick={startCreate} disabled={publishedCards.length===0||isLimited} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-600 px-5 py-3 font-bold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"><Plus aria-hidden="true" className="h-4 w-4"/>{t("Add Your First NFC Tag")}</button><button type="button" onClick={()=>setShowHow(true)} className={actionClass}>{t("Learn How NFC Works")}</button></div>{isLimited&&<p className="mt-3 text-sm text-slate-600">{t("Your current plan does not include another NFC tag.")}<Link href="/dashboard/billing" className="font-bold text-blue-700 underline">{t("Upgrade Plan")}</Link></p>}</div></div></section>:<>
        <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="text-xl font-bold text-slate-950">{t("Your NFC Tags")}</h2><p className="mt-1 text-sm text-slate-500">{t("Tap links point to published cards and update as your tag activity is recorded.")}</p></div><button type="button" onClick={startCreate} disabled={publishedCards.length===0||isLimited} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-600 px-4 py-2.5 text-sm font-bold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"><Plus aria-hidden="true" className="h-4 w-4"/>{t("Add NFC Tag")}</button></div>
        {isLimited&&<p className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900">{tags.data.limit===0?t("NFC tags are not included in your current plan."):t("You have reached the NFC tag limit for your current plan.")} <Link href="/dashboard/billing" className="font-bold underline">{t("Upgrade Plan")}</Link></p>}
        {tags.data.data.length>0&&<div className="grid gap-4 md:grid-cols-2">{tags.data.data.map(tag=><article key={tag.id} className="min-w-0 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:p-6"><div className="flex items-start justify-between gap-3"><div className="flex min-w-0 items-start gap-3"><span className="grid h-11 w-11 shrink-0 place-items-center rounded-2xl bg-cyan-50 text-cyan-700"><Nfc aria-hidden="true" className="h-5 w-5"/></span><div className="min-w-0"><h3 className="break-words text-lg font-bold text-slate-950">{tag.name}</h3><p className="mt-1 break-words text-sm text-slate-500">{tag.template_name?`${tag.card_name} · ${tag.template_name}`:tag.card_name}</p></div></div><span className={`shrink-0 rounded-full px-2.5 py-1 text-xs font-bold ${tag.status==="active"?"bg-emerald-50 text-emerald-700":"bg-slate-100 text-slate-600"}`}>{tag.status==="active"?t("Active"):t("Inactive")}</span></div>
          {tag.card_status!=="published"&&<div className="mt-4 flex flex-wrap items-center justify-between gap-2 rounded-xl bg-amber-50 p-3 text-sm text-amber-900"><span>{t("The connected card is not published.")}</span><Link href={`/dashboard/cards/${tag.card_id}/edit`} className="font-bold underline">{t("Edit Card")}</Link></div>}
          <dl className="mt-5 grid grid-cols-2 gap-x-4 gap-y-4 text-sm"><div className="min-w-0"><dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">{t("Connected Card")}</dt><dd className="mt-1 truncate font-semibold text-slate-800">{tag.card_name}</dd></div><div><dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">{t("Tag ID")}</dt><dd className="mt-1 font-mono font-semibold text-slate-700"><bdi dir="ltr">{tag.tag_hint}</bdi></dd></div><div className="col-span-2 min-w-0"><dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">{t("Public Destination")}</dt><dd className="mt-1 break-all font-mono text-xs text-blue-700"><bdi dir="ltr">{tag.redirect_url}</bdi></dd></div><div><dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">{t("Tap Count")}</dt><dd className="mt-1 font-bold text-slate-900">{tag.tap_count.toLocaleString(language === "ar" ? "ar" : "en")}</dd></div><div><dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">{t("Last Tap")}</dt><dd className="mt-1 text-slate-700">{dateLabel(tag.last_tapped_at,language,t("No taps yet"))}</dd></div><div><dt className="text-xs font-semibold uppercase tracking-wide text-slate-400">{t("Created")}</dt><dd className="mt-1 text-slate-700">{dateLabel(tag.created_at,language,t("No taps yet"))}</dd></div></dl>
          <div className="mt-5 flex flex-wrap gap-2 border-t border-slate-100 pt-4"><Link href={`/c/${encodeURIComponent(tag.card_slug)}`} target="_blank" rel="noreferrer" className={actionClass}><ExternalLink aria-hidden="true" className="h-4 w-4"/>{t("View Card")}</Link><button type="button" onClick={()=>void copyLink(tag.redirect_url)} className={actionClass}><Copy aria-hidden="true" className="h-4 w-4"/>{t("Copy Link")}</button><button type="button" onClick={()=>testLink(tag.redirect_url)} className={actionClass}><Smartphone aria-hidden="true" className="h-4 w-4"/>{t("Test Link")}</button><button type="button" onClick={()=>startEdit(tag)} className={actionClass}><Pencil aria-hidden="true" className="h-4 w-4"/>{t("Edit")}</button><button type="button" onClick={()=>void toggle(tag)} className={actionClass}>{tag.status==="active"?t("Disable"):t("Enable")}</button><button type="button" onClick={()=>void archive(tag)} className="inline-flex min-h-10 items-center justify-center rounded-xl px-3 py-2 text-sm font-semibold text-slate-500 hover:bg-red-50 hover:text-red-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500">{t("Archive")}</button></div>
        </article>)}</div>}
      </>}
    </>}

    {mode&&<Dialog title={editing?t("Edit NFC Tag"):t("Add NFC Tag")} onClose={closeDialog}>
      {created?<div className="mt-5"><div className="rounded-2xl border border-emerald-200 bg-emerald-50 p-4"><p className="flex items-center gap-2 font-bold text-emerald-800"><Check aria-hidden="true" className="h-5 w-5"/>{t("NFC Tag Added")}</p><p className="mt-2 text-sm text-emerald-900">{t("Your NFC tag is connected to")} <strong>{created.card_name}</strong>.</p></div><label className="mt-5 block text-xs font-bold uppercase tracking-wide text-slate-500">{t("NFC sharing URL")}<input dir="ltr" readOnly value={created.redirect_url} className={`${inputClass} font-mono text-xs`} onFocus={event=>event.currentTarget.select()}/></label><div className="mt-4 flex flex-col gap-2 sm:flex-row"><button type="button" onClick={()=>void copyLink(created.redirect_url)} className="inline-flex min-h-11 flex-1 items-center justify-center gap-2 rounded-xl bg-blue-600 px-4 py-3 font-bold text-white hover:bg-blue-700"><Copy aria-hidden="true" className="h-4 w-4"/>{t("Copy NFC Link")}</button><button type="button" onClick={()=>testLink(created.redirect_url)} className={actionClass}><ExternalLink aria-hidden="true" className="h-4 w-4"/>{t("Open Link")}</button><Link href={`/c/${encodeURIComponent(created.card_slug)}`} target="_blank" rel="noreferrer" className={actionClass}>{t("Open Card")}</Link></div>{notice&&<p role="status" className="mt-3 text-sm font-semibold text-emerald-700">{t(notice)}</p>}{error&&<p role="alert" className="mt-3 text-sm text-red-700">{localizeError(error)}</p>}<SetupInstructions/><div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-between"><button type="button" onClick={startCreate} className={actionClass}>{t("Add Another Tag")}</button><button type="button" onClick={closeDialog} className="min-h-11 rounded-xl bg-slate-900 px-5 py-2.5 font-bold text-white">{t("Done")}</button></div></div>:<form onSubmit={saveTag} className="mt-5">
        {!editing&&<div className="mb-5 flex items-center gap-2">{["Name Your Tag","Choose Card","Setup & Test"].map((label,index)=><div key={label} className="flex min-w-0 flex-1 items-center gap-2"><span className={`grid h-7 w-7 shrink-0 place-items-center rounded-full text-xs font-bold ${step>=index+1?"bg-blue-600 text-white":"bg-slate-100 text-slate-500"}`}>{index+1}</span><span className="hidden truncate text-xs font-semibold text-slate-600 sm:block">{t(label)}</span>{index<2&&<span className="h-px flex-1 bg-slate-200"/>}</div>)}</div>}
        {error&&<p role="alert" className="mb-4 rounded-xl bg-red-50 p-3 text-sm text-red-800">{localizeError(error)}{/NFC tag limit/i.test(error)&&<Link href="/dashboard/billing" className="ms-2 font-bold underline">{t("Upgrade Plan")}</Link>}</p>}
        {(editing||step===1)&&<><label className="block text-sm font-semibold text-slate-700">{t("Tag Name *")}<input required maxLength={150} value={form.name} onChange={event=>setForm({...form,name:event.target.value})} placeholder={t("My Business NFC Card")} className={inputClass}/></label>{!editing&&<label className="mt-4 block text-sm font-semibold text-slate-700">{t("NFC Tag UID / Identifier")}<span className="font-normal text-slate-400">{t("(optional)")}</span><input maxLength={200} dir="ltr" value={form.tag_uid} onChange={event=>setForm({...form,tag_uid:event.target.value})} placeholder={t("04:A1:B2:C3:D4")} className={inputClass}/><span className="mt-1.5 block text-xs font-normal text-slate-500">{t("Leave empty if you do not have an identifier. This does not write anything to the physical tag.")}</span></label>}</>}
        {(editing||step===2)&&<div className={editing?"mt-4":""}><label className="block text-sm font-semibold text-slate-700">{t("Assigned Digital Card *")}<select required value={form.card_id} onChange={event=>setForm({...form,card_id:event.target.value})} className={inputClass}><option value="">{t("Choose one of your published cards")}</option>{publishedCards.map(card=><option key={card.id} value={card.id}>{card.name} {t("· Published")}{card.template_name?` · ${card.template_name}`:""}</option>)}</select></label>{publishedCards.length===0&&<p className="mt-2 text-sm text-amber-800">{t("There are no published cards available.")} <Link className="font-bold underline" href="/dashboard/cards/new">{t("Create a card")}</Link> {t("and publish it first.")}</p>}</div>}
        {(editing||step===3)&&<div className="mt-4"><label className="block text-sm font-semibold text-slate-700">{t("Status")}<select value={form.status} onChange={event=>setForm({...form,status:event.target.value as FormValue["status"]})} className={inputClass}><option value="active">{t("Active")}</option><option value="disabled">{t("Inactive")}</option></select></label><div className="mt-4 rounded-xl bg-blue-50 p-4 text-sm leading-6 text-blue-950"><p className="font-bold">{t("Configure the physical tag separately")}</p><p className="mt-1">{t("After saving, copy the NFC link and use a compatible NFC-writing device or application to write it to your tag. Dbizcard does not write NFC hardware from this page.")}</p></div><SetupInstructions compact/></div>}
        <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-between">{editing?<><button type="button" onClick={closeDialog} className={actionClass}>{t("Cancel")}</button><button type="submit" disabled={saving||!publishedCards.length} className="min-h-11 rounded-xl bg-blue-600 px-5 py-2.5 font-bold text-white hover:bg-blue-700 disabled:opacity-50">{saving?t("Saving…"):t("Save Changes")}</button></>:<><button type="button" onClick={()=>step===1?closeDialog():setStep(step-1)} className={actionClass}>{step===1?t("Cancel"):t("Back")}</button>{step<3?<button type="button" onClick={()=>{if(step===1&&!form.name.trim()){setError("Enter a name for this NFC tag.");return;}if(step===2&&!form.card_id){setError("Choose one of your published cards.");return;}setError("");setStep(step+1);}} className="min-h-11 rounded-xl bg-blue-600 px-5 py-2.5 font-bold text-white hover:bg-blue-700">{t("Continue")}<ArrowRight aria-hidden="true" className="ms-1 inline h-4 w-4"/></button>:<button type="submit" disabled={saving||!publishedCards.length||isLimited} className="min-h-11 rounded-xl bg-blue-600 px-5 py-2.5 font-bold text-white hover:bg-blue-700 disabled:opacity-50">{saving?t("Adding…"):t("Add NFC Tag")}</button>}</>}</div>
      </form>}
    </Dialog>}

    {showHow&&<Dialog title={t("How NFC Works")} onClose={()=>setShowHow(false)} wide><div className="mt-5 grid gap-5 md:grid-cols-[.9fr_1.1fr] md:items-center"><Image src="/features/nfc-sharing.webp" alt={t("NFC card and compatible phone sharing a digital business card.")} width={800} height={600} sizes="(max-width: 768px) 100vw, 420px" className="h-auto w-full rounded-[20px] bg-blue-50"/><div><p className="leading-7 text-slate-600">{t("NFC stands for Near Field Communication. Compatible devices can exchange small amounts of information when they are very close together. For Dbizcard, an NFC tag points visitors to your published digital card.")}</p><ol className="mt-4 space-y-3">{["Create and publish your digital card.","Add an NFC tag record in Dbizcard and connect it to the card.","Copy the supplied NFC sharing URL.","Use a compatible NFC-writing device or application to program the physical tag.","Tap with another compatible phone to test the link.","The public digital card opens in the phone’s browser."].map((item,index)=><li key={item} className="flex gap-3 text-sm leading-6 text-slate-700"><span className="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-blue-100 text-xs font-bold text-blue-700">{index+1}</span>{t(item)}</li>)}</ol><p className="mt-4 rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs leading-5 text-amber-900"><Info aria-hidden="true" className="me-1 inline h-4 w-4"/>{t("Hardware and browser support varies. Locking a tag may make it permanently read-only; do so only if you understand that consequence. Dbizcard does not perform universal browser-based NFC writing.")}</p><div className="mt-5"><Link href="/dashboard/cards/new" className="inline-flex min-h-11 items-center justify-center rounded-xl bg-blue-600 px-5 py-3 font-bold text-white hover:bg-blue-700">{t("Create a Digital Card")}</Link></div></div></div><div className="mt-5 rounded-2xl bg-slate-50 p-4 text-center"><div className="flex flex-wrap items-center justify-center gap-3 text-sm font-bold text-slate-800"><span className="rounded-xl border bg-white px-4 py-3">{t("NFC CARD")}</span><span className="text-cyan-600">)))</span><span className="text-cyan-600">)))</span><span className="rounded-xl border bg-white px-4 py-3">{t("PHONE")}</span><ArrowRight aria-hidden="true" className="h-4 w-4 text-blue-600"/><span className="rounded-xl bg-blue-100 px-4 py-3 text-blue-900">{t("DIGITAL CARD")}</span></div></div></Dialog>}
  </div>;
}

function Metric({label,value,icon:Icon}:{label:string;value:string;icon:typeof Nfc}){
 const { t } = useCustomerLocale();
return <div className="flex items-center gap-3 rounded-2xl border border-slate-200 bg-white p-4 shadow-sm"><span className="grid h-10 w-10 place-items-center rounded-xl bg-blue-50 text-blue-700"><Icon aria-hidden="true" className="h-5 w-5"/></span><div><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{t(label)}</p><p className="mt-0.5 text-xl font-black text-slate-950">{value}</p></div></div>}

function SetupInstructions({compact=false}:{compact?:boolean}){
 const { t } = useCustomerLocale();
return <div className={`mt-5 rounded-2xl border border-cyan-100 bg-cyan-50/60 ${compact?"p-4":"p-5"}`}><h3 className="font-bold text-slate-950">{t("Set up your physical NFC tag")}</h3><ol className="mt-3 space-y-2 text-sm leading-6 text-slate-700">{["Copy your NFC link.","Open a compatible NFC writer application or use a compatible NFC-writing device.","Write the URL to your NFC card or tag.","Lock the tag only if you understand it may become permanently read-only; this is optional.","Tap with another compatible phone and use Test Link to verify the destination."].map((item,index)=><li key={item} className="flex gap-2"><span className="font-bold text-cyan-800">{index+1}.</span><span>{t(item)}</span></li>)}</ol><p className="mt-3 text-xs leading-5 text-slate-500">{t("The Test Link opens the redirect in a browser; it does not simulate or count a physical NFC tap.")}</p></div>}

function dateLabel(value:string|null,language:string,empty:string){return value?new Date(value).toLocaleString(language === "ar" ? "ar" : "en",{year:"numeric",month:"short",day:"numeric",hour:"numeric",minute:"2-digit"}):empty;}
