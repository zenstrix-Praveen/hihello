"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { adminApi } from "@/lib/admin-api";
import {useAdminLocation} from "@/lib/admin-location";
import { BadgeDollarSign, Building2, CreditCard, Plus, Users } from "lucide-react";
import {formatCurrencyAmount} from "@/lib/global-pricing";
import GlobalPlanPrices from "@/components/admin/GlobalPlanPrices";
import PlanCurrencySelect from "@/components/admin/PlanCurrencySelect";
import { useAdminLocale } from "@/lib/admin-locale";
import { PlanEntitlementsEditor } from "@/components/admin/PlanEntitlementsEditor";
import { mergeEntitlementStorage, type FeatureDefinition, type PlanEntitlement } from "@/lib/plan-entitlements";
import PlanConfigurationSections from "@/components/admin/PlanConfigurationSections";

type Plan={id:number;name:string;code:string;description:string;price:number;currency:string;billing_period:string;trial_days:number;trial_mode:"disabled"|"limited"|"unlimited";visibility:"public"|"hidden";billing_type:"per_user"|"per_workspace";is_most_popular:boolean;features:string[];limits:Record<string,unknown>;is_featured:boolean;is_active:boolean;sort_order:number;subscriber_count:number;has_history:boolean;created_at:string;updated_at:string};
type Sub={id:number;plan_id:number|null;user_id:number|null;organization_id:number|null;status:string;amount:number;currency:string;billing_period:string;starts_at:string;ends_at:string|null;trial_ends_at:string|null;created_at:string;plan_name:string;user_name:string;user_email:string;organization_name:string};
type Summary={total_plans:number;active_plans:number;total_subscribers:number;active_subscriptions:number;trial_subscriptions:number;monthly_revenue:Record<string,number>;annual_revenue:Record<string,number>};
type PlanSubscriber={id:number;status:string;amount:number;currency:string;user_name:string;user_email:string;organization_name:string};
type PaymentRow={id:number;public_id:string;subscriber_name:string;subscriber_email:string;organization_name:string;plan_name:string;amount:number;currency:string;provider:string;provider_order_id:string|null;payment_method:string|null;status:string;provider_payment_id:string|null;paid_at:string|null;created_at:string};
type PaymentSummary={total_payment_attempts:number;successful_payments:number;failed_payments:number;pending_payments:number;verified_revenue:Record<string,number>};
type PaymentDetails=PaymentRow&{organization_id:number|null;subscription_id:number|null;bank:string|null;wallet:string|null;error_code:string|null;error_description:string|null;updated_at:string;subscription:{status:string;started_at:string|null;ends_at:string|null};transactions:{id:number;event_type:string;amount:number;currency:string;created_at:string;payload:unknown}[]};
type PaymentTransaction={id:number;provider_event_id:string|null;payment_id:number;payment_public_id:string;subscriber_name:string;subscriber_email:string;event_type:string;amount:number;currency:string;status:string;created_at:string};
type TransactionDetails=Omit<PaymentTransaction,"status">&{status?:string;payment_status:string;payload:unknown};
type User={id:number;name:string;email:string}; type Org={id:number;name:string};
const field="mt-1 w-full rounded-xl border border-slate-300 px-3 py-2.5";
const blankPlan=()=>({name:"",code:"",description:"",price:0,currency:"USD",billing_period:"monthly",trial_days:0,trial_mode:"disabled" as "disabled"|"limited"|"unlimited",visibility:"public" as "public"|"hidden",billing_type:"per_user" as "per_user"|"per_workspace",is_most_popular:false,features:[] as string[],limits:{digital_cards_limit:0,digital_cards_unlimited:false,qr_codes_limit:0,qr_codes_unlimited:false,nfc_tags_limit:0,nfc_tags_unlimited:false,contacts_limit:0,contacts_unlimited:false,team_members_limit:0,team_members_unlimited:false,storage_limit:0,storage_unlimited:false,card_templates_limit:0,card_templates_unlimited:false,analytics_level:"disabled",branding_level:"standard"},is_featured:false,is_active:true,sort_order:0});
const newPlan=()=>({...blankPlan(),limits:{...blankPlan().limits,digital_cards_limit:3,nfc_tags_limit:2,qr_codes_limit:10,contacts_limit:100,storage_limit:5,team_members_limit:2,card_templates_limit:5}});
const blankSub=()=>({plan_id:"",user_id:"",organization_id:"",status:"active",starts_at:new Date().toISOString().slice(0,10),ends_at:"",trial_ends_at:"",amount:"",currency:"",billing_period:""});
const date=(s:string|null)=>s?new Date(s).toLocaleDateString():"—";
const revenue=(v:Record<string,number>|undefined)=>{const rows=Object.entries(v??{}).filter(([,n])=>n!==0);return rows.length?rows.map(([currency,n])=>`${currency} ${formatCurrencyAmount(n,currency)}`).join(" · "):"0.00"};

export default function PlansBillingPage(){
 const {t,language}=useAdminLocale();
 const [globalPricePlan,setGlobalPricePlan]=useState<Plan|null>(null);
 const originalPlanCurrency=useRef<string|undefined>(undefined);
 const {search:routeSearch}=useAdminLocation();
 const [tab,setTab]=useState<"plans"|"subscriptions"|"payments"|"transactions">("plans"),[plans,setPlans]=useState<Plan[]>([]),[subs,setSubs]=useState<Sub[]>([]),[summary,setSummary]=useState<Summary|null>(null),[users,setUsers]=useState<User[]>([]),[orgs,setOrgs]=useState<Org[]>([]);
 const [loading,setLoading]=useState(true),[saving,setSaving]=useState(false),[error,setError]=useState(""),[notice,setNotice]=useState(""),[search,setSearch]=useState(""),[statusFilter,setStatusFilter]=useState(""),[planFilter,setPlanFilter]=useState("");
 const [planForm,setPlanForm]=useState<ReturnType<typeof blankPlan>|null>(null),[planEdit,setPlanEdit]=useState<number|null>(null),[viewPlan,setViewPlan]=useState<Plan|null>(null),[viewSubscribers,setViewSubscribers]=useState<PlanSubscriber[]>([]),[subForm,setSubForm]=useState<ReturnType<typeof blankSub>|null>(null),[subEdit,setSubEdit]=useState<number|null>(null),[viewSub,setViewSub]=useState<Sub|null>(null);
 const [entitlementPlan,setEntitlementPlan]=useState<Plan|null>(null);
 const [planActionId,setPlanActionId]=useState<number|null>(null);
 const planActionLock=useRef(false);
 const [payments,setPayments]=useState<PaymentRow[]>([]),[paymentSummary,setPaymentSummary]=useState<PaymentSummary|null>(null),[paymentMethod,setPaymentMethod]=useState(""),[paymentProvider,setPaymentProvider]=useState(""),[paymentCurrency,setPaymentCurrency]=useState(""),[paymentFrom,setPaymentFrom]=useState(""),[paymentTo,setPaymentTo]=useState(""),[viewPayment,setViewPayment]=useState<PaymentDetails|null>(null),[transactions,setTransactions]=useState<PaymentTransaction[]>([]),[viewTransaction,setViewTransaction]=useState<TransactionDetails|null>(null),[billingLoading,setBillingLoading]=useState(false),[billingError,setBillingError]=useState(""),[billingVersion,setBillingVersion]=useState(0),[deletingPayment,setDeletingPayment]=useState<number|null>(null),[cancellingPayment,setCancellingPayment]=useState<number|null>(null);
 const load=useCallback(async()=>{setLoading(true);setError("");try{const [p,s,m]=await Promise.all([adminApi(`/admin/plans?search=${encodeURIComponent(search)}&status=${encodeURIComponent(tab==="plans"?statusFilter:"")}`),adminApi(`/admin/subscriptions?search=${encodeURIComponent(search)}&status=${encodeURIComponent(tab==="subscriptions"?statusFilter:"")}&plan_id=${encodeURIComponent(planFilter)}`),adminApi("/admin/billing/summary")]);setPlans(p.data);setSubs(s.data);setSummary(m)}catch(e){setError(e instanceof Error?e.message:"Could not load plans and billing")}finally{setLoading(false)}},[search,statusFilter,tab,planFilter]);
 useEffect(()=>{void load()},[load]);
 useEffect(()=>{if(tab!=="payments")return;let live=true;setBillingLoading(true);setBillingError("");const q=new URLSearchParams({search,status:statusFilter,provider:paymentProvider,currency:paymentCurrency,payment_method:paymentMethod,plan_id:planFilter,from:paymentFrom,to:paymentTo});Promise.all([adminApi(`/admin/payments?${q}`),adminApi("/admin/payments/summary")]).then(([p,s])=>{if(live){setPayments(p.data??[]);setPaymentSummary(s)}}).catch(e=>{if(live)setBillingError(e instanceof Error?e.message:"Could not load payment history")}).finally(()=>{if(live)setBillingLoading(false)});return()=>{live=false}},[tab,search,statusFilter,paymentMethod,paymentProvider,paymentCurrency,planFilter,paymentFrom,paymentTo,billingVersion]);
 useEffect(()=>{if(tab!=="transactions")return;let live=true;setBillingLoading(true);setBillingError("");const q=new URLSearchParams({search});adminApi(`/admin/transactions?${q}`).then(r=>{if(live)setTransactions(r.data??[])}).catch(e=>{if(live)setBillingError(e instanceof Error?e.message:"Could not load transaction history")}).finally(()=>{if(live)setBillingLoading(false)});return()=>{live=false}},[tab,search]);
 useEffect(()=>{const params=new URLSearchParams(routeSearch);const requested=params.get("tab")||"plans";if(requested==="plans"||requested==="subscriptions"||requested==="payments"||requested==="transactions")setTab(requested);if(params.get("create")==="1"&&requested==="plans"){setPlanForm(newPlan());setPlanEdit(null)}else if(params.get("create")!=="1")setPlanForm(null)},[routeSearch]);
 function selectTab(next:"plans"|"subscriptions"|"payments"|"transactions"){setTab(next);const url=new URL(window.location.href);url.searchParams.set("tab",next);if(next!=="plans")url.searchParams.delete("create");window.history.pushState({},"",url);window.dispatchEvent(new PopStateEvent("popstate"))}
 useEffect(()=>{Promise.all([adminApi("/admin/users?page=1&limit=200"),adminApi("/admin/organizations?page=1&limit=200")]).then(([u,o])=>{setUsers(u.data??[]);setOrgs(o.data??[])}).catch(e=>setError(e instanceof Error?e.message:"Could not load subscribers"))},[]);
 async function savePlan(e:React.FormEvent){e.preventDefault();if(!planForm)return;setSaving(true);setError("");try{const url=planEdit?`/admin/plans/${planEdit}`:"/admin/plans";await adminApi(url,{method:planEdit?"PUT":"POST",body:JSON.stringify({...planForm,price:Number(planForm.price),trial_days:Number(planForm.trial_days),sort_order:Number(planForm.sort_order)})});setPlanForm(null);setPlanEdit(null);setNotice(planEdit?"Plan updated.":"Plan created.");await load()}catch(e){setError(e instanceof Error?e.message:"Could not save plan")}finally{setSaving(false)}}
 function editPlan(p:Plan){originalPlanCurrency.current=p.currency;setPlanEdit(p.id);const limits:Record<string,unknown>={...blankPlan().limits,...(p.limits||{})};limits.analytics_level=String(p.limits?.analytics_level??(p.features.includes("analytics")?"basic":"disabled"));limits.branding_level=String(p.limits?.branding_level??(p.features.includes("custom_branding")?"custom":"standard"));for(const [key,fallback] of [["digital_cards_limit",3],["nfc_tags_limit",2],["qr_codes_limit",10],["contacts_limit",100],["storage_limit",5],["team_members_limit",2],["card_templates_limit",5]] as const){if(Number(limits[key])===-1){limits[key]=fallback;limits[`${key.replace(/_limit$/,"")}_unlimited`]=true}}setPlanForm({...blankPlan(),...p,limits:limits as ReturnType<typeof blankPlan>["limits"]})}
 function syncPlanEntitlements(planId:number,entitlements:Record<string,PlanEntitlement>,registry:FeatureDefinition[]){
  const update=(plan:Plan):Plan=>{
   return {...plan,...mergeEntitlementStorage(plan.features,plan.limits,entitlements,registry)};
  };
  setPlans(current=>current.map(plan=>plan.id===planId?update(plan):plan));
  setViewPlan(current=>current?.id===planId?update(current):current);
 }
 async function showPlan(p:Plan){setViewPlan(p);setViewSubscribers([]);try{const d=await adminApi(`/admin/plans/${p.id}`);setViewPlan(d);setViewSubscribers(d.subscribers??[])}catch(e){setError(e instanceof Error?e.message:"Could not load plan details")}}
 async function showPayment(p:PaymentRow){try{const d=await adminApi(`/admin/payments/${p.id}`);setViewPayment(d.data)}catch(e){setBillingError(e instanceof Error?e.message:"Could not load payment details")}}
 async function deletePayment(p:PaymentRow){if(!["created","pending","failed","cancelled"].includes(p.status))return;if(!window.confirm(`Delete this ${p.status} payment attempt? This is allowed only for unlinked nonfinal records.`))return;setDeletingPayment(p.id);setBillingError("");setNotice("");try{await adminApi(`/admin/payments/${p.id}`,{method:"DELETE"});setNotice("Payment record deleted.");setBillingVersion(v=>v+1)}catch(e){setBillingError(e instanceof Error?e.message:"Could not delete payment record.")}finally{setDeletingPayment(null)}}
 async function cancelPayment(p:PaymentRow){if(!["created","pending"].includes(p.status))return;if(!window.confirm("Cancel this open payment attempt? The provider order is not voided by this action."))return;setCancellingPayment(p.id);setBillingError("");setNotice("");try{await adminApi(`/admin/payments/${p.id}/status`,{method:"PATCH",body:JSON.stringify({status:"cancelled"})});setNotice("Payment attempt marked cancelled.");setViewPayment(null);setBillingVersion(v=>v+1)}catch(e){setBillingError(e instanceof Error?e.message:"Could not cancel payment.")}finally{setCancellingPayment(null)}}
 function printPayment(payment:PaymentDetails){
  const esc=(value:string)=>
   value
    .replaceAll("&","&amp;")
    .replaceAll("<","&lt;")
    .replaceAll(">","&gt;")
    .replaceAll('"',"&quot;")
    .replaceAll("'","&#039;");

  const customerName=
   payment.subscriber_name==="Deleted User"
    ?"Former Customer"
    :(payment.subscriber_name||"—");

  const customerEmail=
   payment.subscriber_email?.endsWith("@deleted.invalid")
    ?"—"
    :(payment.subscriber_email||"—");

  const rows:[string,string][]=[
   ["Customer",customerName],
   ["Customer Email",customerEmail],
   ["Organization",payment.organization_name||"—"],
   ["Plan",payment.plan_name||"—"],
   ["Amount",`${payment.currency} ${formatCurrencyAmount(payment.amount,payment.currency,language)}`],
   ["Provider",payment.provider||"—"],
   ["Payment Method",payment.payment_method||"—"],
   ["Status",payment.status||"—"],
   ["Provider Order ID",payment.provider_order_id||"—"],
   ["Provider Payment ID",payment.provider_payment_id||"—"],
   ["Paid Date",date(payment.paid_at)],
   ["Created Date",date(payment.created_at)],
   ["Subscription",payment.subscription?.status||"—"],
   ["Subscription Start",date(payment.subscription?.started_at||null)],
   ["Subscription End",date(payment.subscription?.ends_at||null)],
   ["Payment Error Code",payment.error_code||"—"],
   ["Payment Error",payment.error_description||"—"]
  ];

  const iframe=document.createElement("iframe");

  iframe.style.position="fixed";
  iframe.style.width="1px";
  iframe.style.height="1px";
  iframe.style.right="0";
  iframe.style.bottom="0";
  iframe.style.border="0";
  iframe.style.opacity="0";

  document.body.appendChild(iframe);

  const w=iframe.contentWindow;
  const d=iframe.contentDocument;

  if(!w||!d){
   iframe.remove();
   setBillingError("Could not prepare payment for printing.");
   return;
  }

  const cards=rows.map(([label,value])=>`
   <div class="card">
    <small>${esc(label)}</small>
    <strong>${esc(value)}</strong>
   </div>
  `).join("");

  const timeline=(payment.transactions??[]).map(x=>`
   <div class="timeline">
    <span><b>${esc(x.event_type)}</b> ${esc(date(x.created_at))}</span>
    <span>${esc(x.currency)} ${formatCurrencyAmount(x.amount,x.currency,language)}</span>
   </div>
  `).join("");

  d.open();

  d.write(`
<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>Payment Details</title>

<style>
@page{
 size:A4 portrait;
 margin:8mm;
}

*{
 box-sizing:border-box;
}

html,body{
 margin:0;
 padding:0;
 background:#fff;
 color:#000;
 font-family:Arial,sans-serif;
}

.receipt{
 width:100%;
 max-width:190mm;
 margin:0 auto;
}

h1{
 margin:0;
 font-size:20px;
}

.ref{
 margin:4px 0 10px;
 font:9px monospace;
}

.grid{
 display:grid;
 grid-template-columns:repeat(3,1fr);
 gap:5px;
}

.card{
 border:1px solid #000;
 border-radius:4px;
 padding:5px 6px;
 min-height:38px;
 page-break-inside:avoid;
 break-inside:avoid;
}

.card small{
 display:block;
 font-size:7px;
 margin-bottom:2px;
}

.card strong{
 display:block;
 font-size:9px;
 line-height:1.15;
 overflow-wrap:anywhere;
}

h2{
 margin:9px 0 4px;
 font-size:11px;
}

.timeline{
 border:1px solid #000;
 border-radius:4px;
 padding:5px 6px;
 font-size:8px;
 display:flex;
 justify-content:space-between;
 gap:8px;
 page-break-inside:avoid;
 break-inside:avoid;
}
</style>
</head>

<body>
<main class="receipt">

<h1>Payment Details</h1>

<div class="ref">
${esc(payment.public_id)}
</div>

<div class="grid">
${cards}
</div>

<h2>Payment Timeline</h2>

${timeline||'<div class="timeline">No payment events recorded.</div>'}

</main>
</body>
</html>
  `);

  d.close();

  const cleanup=()=>{
   setTimeout(()=>{
    if(iframe.isConnected)iframe.remove();
   },300);
  };

  w.addEventListener("afterprint",cleanup,{once:true});

  setTimeout(()=>{
   try{
    w.focus();
    w.print();
   }catch{
    setBillingError("Could not open payment print dialog.");
    cleanup();
   }
  },300);

  setTimeout(cleanup,5000);
 }
 async function showTransaction(p:PaymentTransaction){try{const d=await adminApi(`/admin/transactions/${p.id}`);setViewTransaction(d.data)}catch(e){setBillingError(e instanceof Error?e.message:"Could not load transaction details")}}
 async function removePlan(p:Plan){if(planActionLock.current)return;planActionLock.current=true;const message=p.has_history?"This plan has existing subscribers or billing history. Deleting it will remove it from future use, while existing subscriptions and billing history will be preserved. Continue?":"Delete this plan? It will be archived and removed from future use.";if(!window.confirm(message)){planActionLock.current=false;return}setPlanActionId(p.id);setError("");setNotice("");try{await adminApi(`/admin/plans/${p.id}`,{method:"DELETE"});setNotice("Plan archived.");await load()}catch(e){setError(e instanceof Error?e.message:"Could not archive plan")}finally{planActionLock.current=false;setPlanActionId(null)}}
 async function togglePlan(p:Plan){if(planActionLock.current)return;planActionLock.current=true;setPlanActionId(p.id);setError("");setNotice("");try{await adminApi(`/admin/plans/${p.id}/status`,{method:"PATCH",body:JSON.stringify({is_active:!p.is_active})});setNotice(`Plan ${p.is_active?"deactivated":"activated"}.`);await load()}catch(e){setError(e instanceof Error?e.message:"Could not update plan")}finally{planActionLock.current=false;setPlanActionId(null)}}
 async function saveSub(e:React.FormEvent){e.preventDefault();if(!subForm)return;setSaving(true);setError("");try{const payload={...subForm,plan_id:Number(subForm.plan_id),user_id:subForm.user_id?Number(subForm.user_id):null,organization_id:subForm.organization_id?Number(subForm.organization_id):null,amount:subForm.amount===""?null:Number(subForm.amount)};await adminApi(subEdit?`/admin/subscriptions/${subEdit}`:"/admin/subscriptions",{method:subEdit?"PUT":"POST",body:JSON.stringify(payload)});setSubForm(null);setSubEdit(null);setNotice(subEdit?"Subscription updated.":"Subscription created.");await load()}catch(e){setError(e instanceof Error?e.message:"Could not save subscription")}finally{setSaving(false)}}
 function editSub(s:Sub){setSubEdit(s.id);setSubForm({plan_id:String(s.plan_id??""),user_id:String(s.user_id??""),organization_id:String(s.organization_id??""),status:s.status,starts_at:s.starts_at?.slice(0,10)??"",ends_at:s.ends_at?.slice(0,10)??"",trial_ends_at:s.trial_ends_at?.slice(0,10)??"",amount:String(s.amount),currency:s.currency,billing_period:s.billing_period})}
 async function subStatus(s:Sub,status:string){if(status==="cancelled"&&!window.confirm("Cancel this subscription? No payment will be charged."))return;try{await adminApi(`/admin/subscriptions/${s.id}/status`,{method:"PATCH",body:JSON.stringify({status})});setNotice(`Subscription marked ${status.replace("_"," ")}.`);await load()}catch(e){setError(e instanceof Error?e.message:"Could not update subscription")}}
 const metrics=[ ["Total Plans",summary?.total_plans,BadgeDollarSign],["Active Plans",summary?.active_plans,BadgeDollarSign],["Total Subscribers",summary?.total_subscribers,Users],["Active Subscriptions",summary?.active_subscriptions,CreditCard],["Trial Subscriptions",summary?.trial_subscriptions,Users],["Monthly Revenue",summary?revenue(summary.monthly_revenue):undefined,BadgeDollarSign],["Annual Revenue",summary?revenue(summary.annual_revenue):undefined,BadgeDollarSign] ] as const;
 const paymentsPanel=<div className="mt-4 space-y-4">{billingError&&<div role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{billingError}</div>}<div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">{[["Total Payment Attempts",paymentSummary?.total_payment_attempts??"—"],["Successful Payments",paymentSummary?.successful_payments??"—"],["Failed Payments",paymentSummary?.failed_payments??"—"],["Pending Payments",paymentSummary?.pending_payments??"—"],["Verified Revenue",paymentSummary?revenue(paymentSummary.verified_revenue):"—"]].map(([label,value])=><div key={label} className="rounded-xl border bg-white p-4"><p className="text-xs text-slate-500">{t(String(label))}</p><p className="mt-2 text-lg font-bold">{value}</p></div>)}</div><div className="flex flex-wrap gap-2 rounded-xl border bg-white p-3"><input value={search} onChange={e=>setSearch(e.target.value)} placeholder={t("Search payments")} className="min-w-52 flex-1 rounded-lg border px-3 py-2 text-sm"/><select value={statusFilter} onChange={e=>setStatusFilter(e.target.value)} className="rounded-lg border px-3 py-2 text-sm"><option value="">{t("All statuses")}</option>{["created","pending","authorized","paid","failed","cancelled","refunded"].map(v=><option key={v}>{t(v)}</option>)}</select><select value={paymentProvider} onChange={e=>setPaymentProvider(e.target.value)} className="rounded-lg border px-3 py-2 text-sm"><option value="">{t("All providers")}</option>{["razorpay","mock"].map(v=><option key={v}>{v}</option>)}</select><input aria-label="Currency filter" value={paymentCurrency} maxLength={3} onChange={e=>setPaymentCurrency(e.target.value.toUpperCase())} placeholder="Currency" className="w-28 rounded-lg border px-3 py-2 text-sm"/><select value={paymentMethod} onChange={e=>setPaymentMethod(e.target.value)} className="rounded-lg border px-3 py-2 text-sm"><option value="">{t("All payment methods")}</option>{["upi","netbanking","card","wallet"].map(v=><option key={v} value={v}>{t(v)}</option>)}</select><select value={planFilter} onChange={e=>setPlanFilter(e.target.value)} className="rounded-lg border px-3 py-2 text-sm"><option value="">{t("All plans")}</option>{plans.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select><input aria-label={t("From date")} type="date" value={paymentFrom} onChange={e=>setPaymentFrom(e.target.value)} className="rounded-lg border px-3 py-2 text-sm"/><input aria-label={t("To date")} type="date" value={paymentTo} onChange={e=>setPaymentTo(e.target.value)} className="rounded-lg border px-3 py-2 text-sm"/></div><div className="overflow-x-auto rounded-2xl border bg-white">{billingLoading?<p className="p-10 text-center text-sm text-slate-500">{t("Loading payments…")}</p>:payments.length===0?<p className="p-10 text-center text-sm text-slate-500">{t("No payments found")}</p>:<table className="w-full min-w-[1400px] text-left text-sm"><thead className="bg-slate-50 text-slate-500"><tr>{["Payment ID","Subscriber","Plan","Amount","Currency","Provider","Payment Method","Status","Provider Order ID","Provider Payment ID","Paid At","Created At","Actions"].map(x=><th key={x} className="px-3 py-3 font-medium">{t(x)}</th>)}</tr></thead><tbody className="divide-y">{payments.map(p=><tr key={p.id}><td className="px-3 py-3 font-mono text-xs">{p.public_id}</td><td className="px-3 py-3">{p.subscriber_name}<div className="text-xs text-slate-500">{p.subscriber_email}</div></td><td className="px-3 py-3">{p.plan_name}</td><td className="px-3 py-3">{formatCurrencyAmount(p.amount,p.currency,language)}</td><td className="px-3 py-3">{p.currency}</td><td className="px-3 py-3">{p.provider}</td><td className="px-3 py-3">{p.payment_method?t(p.payment_method):"—"}</td><td className="px-3 py-3">{t(p.status)}</td><td className="px-3 py-3 font-mono text-xs">{p.provider_order_id||"—"}</td><td className="px-3 py-3 font-mono text-xs">{p.provider_payment_id||"—"}</td><td className="px-3 py-3">{p.paid_at?date(p.paid_at):"—"}</td><td className="px-3 py-3">{date(p.created_at)}</td><td className="px-3 py-3"><div className="flex flex-wrap gap-2"><button onClick={()=>void showPayment(p)} className="rounded border px-2 py-1">View</button>{["created","pending"].includes(p.status)&&<button disabled={cancellingPayment===p.id} onClick={()=>void cancelPayment(p)} className="rounded border px-2 py-1 text-amber-700 disabled:opacity-50">{cancellingPayment===p.id?"Cancelling…":"Cancel"}</button>}{["failed","cancelled"].includes(p.status)&&<button disabled={deletingPayment===p.id} onClick={()=>void deletePayment(p)} className="rounded border px-2 py-1 text-red-700 disabled:opacity-50">{deletingPayment===p.id?"Deleting…":"Delete"}</button>}</div></td></tr>)}</tbody></table>}</div></div>;
 const transactionsPanel=<div className="mt-4 space-y-4">{billingError&&<div role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{billingError}</div>}<input value={search} onChange={e=>setSearch(e.target.value)} placeholder={t("Search transactions")} className="w-full rounded-lg border bg-white px-3 py-2 text-sm"/><div className="overflow-x-auto rounded-2xl border bg-white">{billingLoading?<p className="p-10 text-center text-sm text-slate-500">{t("Loading transactions…")}</p>:transactions.length===0?<p className="p-10 text-center text-sm text-slate-500">{t("No transactions found")}</p>:<table className="w-full min-w-[900px] text-left text-sm"><thead className="bg-slate-50 text-slate-500"><tr>{["Event ID","Payment","Subscriber","Event Type","Amount","Currency","Status","Created At","Actions"].map(x=><th key={x} className="px-4 py-3 font-medium">{t(x)}</th>)}</tr></thead><tbody className="divide-y">{transactions.map(x=><tr key={x.id}><td className="px-4 py-3 font-mono text-xs">{x.provider_event_id||x.id}</td><td className="px-4 py-3 font-mono text-xs">{x.payment_public_id}</td><td className="px-4 py-3">{x.subscriber_name}<div className="text-xs text-slate-500">{x.subscriber_email}</div></td><td className="px-4 py-3">{x.event_type}</td><td className="px-4 py-3">{formatCurrencyAmount(x.amount,x.currency,language)}</td><td className="px-4 py-3">{x.currency}</td><td className="px-4 py-3">{t(x.status)}</td><td className="px-4 py-3">{date(x.created_at)}</td><td className="px-4 py-3"><button onClick={()=>void showTransaction(x)} className="rounded border px-2 py-1">{t("View Details")}</button></td></tr>)}</tbody></table>}</div></div>;
 return <section className="max-w-7xl"><p className="text-sm text-slate-500">Platform management</p><div className="mt-1 flex flex-wrap items-end justify-between gap-4"><div><h1 className="text-3xl font-bold">Plans &amp; Billing</h1><p className="mt-2 text-sm text-slate-500">Manage subscription plans, limits and platform subscriptions.</p></div>{tab==="plans"?<button onClick={()=>{setPlanForm(newPlan());setPlanEdit(null)}} className="inline-flex items-center gap-2 rounded-xl bg-blue-600 px-4 py-2.5 font-semibold text-white"><Plus size={18}/>Add Plan</button>:<button onClick={()=>{setSubForm(blankSub());setSubEdit(null)}} className="inline-flex items-center gap-2 rounded-xl bg-blue-600 px-4 py-2.5 font-semibold text-white"><Plus size={18}/>Add Subscription</button>}</div>
 {notice&&<div role="status" className="mt-5 rounded-xl border border-green-200 bg-green-50 p-3 text-green-800">{notice}</div>}{error&&<div role="alert" className="mt-5 rounded-xl border border-red-200 bg-red-50 p-3 text-red-800">{error}<button onClick={()=>void load()} className="ml-3 font-semibold underline">Retry</button></div>}
 <div className="mt-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">{metrics.map(([label,value,Icon])=><div key={label} className="rounded-2xl border bg-white p-4"><div className="flex items-center justify-between text-sm text-slate-500"><span>{label}</span><Icon size={17}/></div><p className="mt-3 text-2xl font-bold">{value===undefined?"—":value}</p>{(label==="Monthly Revenue"||label==="Annual Revenue")&&<p className="mt-1 text-xs text-slate-500">Recurring run rate from active monthly/yearly subscriptions</p>}</div>)}</div>
 <div className="mt-7 flex flex-wrap items-center gap-3 border-b"><button onClick={()=>selectTab("plans")} className={`border-b-2 px-4 py-3 text-sm font-semibold ${tab==="plans"?"border-blue-600 text-blue-700":"border-transparent text-slate-500"}`}>Plans</button><button onClick={()=>selectTab("subscriptions")} className={`border-b-2 px-4 py-3 text-sm font-semibold ${tab==="subscriptions"?"border-blue-600 text-blue-700":"border-transparent text-slate-500"}`}>Subscriptions</button><button onClick={()=>selectTab("payments")} className={`border-b-2 px-4 py-3 text-sm font-semibold ${tab==="payments"?"border-blue-600 text-blue-700":"border-transparent text-slate-500"}`}>Payments</button><button onClick={()=>selectTab("transactions")} className={`border-b-2 px-4 py-3 text-sm font-semibold ${tab==="transactions"?"border-blue-600 text-blue-700":"border-transparent text-slate-500"}`}>Transactions</button><div className="ml-auto flex flex-wrap gap-2 pb-2"><input value={search} onChange={e=>setSearch(e.target.value)} placeholder={tab==="plans"?"Search plans":"Search subscribers"} className="rounded-lg border px-3 py-2 text-sm"/><select value={statusFilter} onChange={e=>setStatusFilter(e.target.value)} className="rounded-lg border px-3 py-2 text-sm"><option value="">All statuses</option>{tab==="plans"?<><option value="active">Active</option><option value="inactive">Inactive</option></>:<>{["trial","active","past_due","cancelled","expired"].map(s=><option key={s}>{s}</option>)}</>}</select>{tab==="subscriptions"&&<select value={planFilter} onChange={e=>setPlanFilter(e.target.value)} className="rounded-lg border px-3 py-2 text-sm"><option value="">All plans</option>{plans.map(p=><option value={p.id} key={p.id}>{p.name}</option>)}</select>}</div></div>
 {loading?<div className="mt-6 rounded-2xl border bg-white p-12 text-center text-slate-500">Loading plans and subscriptions…</div>:tab==="payments"?paymentsPanel:tab==="transactions"?transactionsPanel:tab==="plans"?<div className="mt-4 overflow-hidden rounded-2xl border bg-white">{plans.length===0?<div className="p-12 text-center"><p className="font-semibold">No plans found</p><p className="mt-1 text-sm text-slate-500">Create a plan to begin managing platform subscriptions.</p><button onClick={()=>{setPlanForm(newPlan());setPlanEdit(null)}} className="mt-4 rounded-xl bg-blue-600 px-4 py-2 font-semibold text-white">+ Add Plan</button></div>:<div className="overflow-x-auto"><table className="w-full min-w-[950px] text-left text-sm"><thead className="bg-slate-50 text-slate-500"><tr>{["Plan Name","Code","Price","Period","Featured","Status","Subscribers","Sort Order","Actions"].map(x=><th key={x} className="px-4 py-3 font-medium">{x}</th>)}</tr></thead><tbody className="divide-y">{plans.map(p=><tr key={p.id}><td className="px-4 py-3"><div className="font-semibold">{p.name}</div><div className="text-xs text-slate-500">{p.description}</div></td><td className="px-4 py-3 font-mono">{p.code}</td><td className="px-4 py-3">{p.currency} {formatCurrencyAmount(p.price,p.currency,language)}</td><td className="px-4 py-3 capitalize">{p.billing_period}</td><td className="px-4 py-3">{p.is_featured?"Yes":"No"}</td><td className="px-4 py-3"><span className={`rounded-full px-2.5 py-1 text-xs ${p.is_active?"bg-green-50 text-green-700":"bg-slate-100 text-slate-600"}`}>{p.is_active?"Active":"Inactive"}</span></td><td className="px-4 py-3">{p.subscriber_count}</td><td className="px-4 py-3">{p.sort_order}</td><td className="px-4 py-3"><div className="flex gap-2"><button onClick={()=>void showPlan(p)} className="rounded border px-2 py-1">View</button><button onClick={()=>editPlan(p)} className="rounded border px-2 py-1">Edit</button><button onClick={()=>setGlobalPricePlan(p)} className="rounded border px-2 py-1">{language==="ar"?"أسعار عالمية":"Global prices"}</button><button onClick={()=>setEntitlementPlan(p)} className="rounded border px-2 py-1">Entitlements</button><button onClick={()=>void togglePlan(p)} disabled={planActionId!==null} className="rounded border px-2 py-1">{p.is_active?"Deactivate":"Activate"}</button><button onClick={()=>void removePlan(p)} disabled={planActionId!==null} className="rounded border px-2 py-1 text-red-700">Delete</button></div></td></tr>)}</tbody></table></div>}</div>:<div className="mt-4 overflow-hidden rounded-2xl border bg-white">{subs.length===0?<div className="p-12 text-center"><p className="font-semibold">No subscriptions found</p><p className="mt-1 text-sm text-slate-500">Subscriptions created by administrators will appear here.</p></div>:<div className="overflow-x-auto"><table className="w-full min-w-[1100px] text-left text-sm"><thead className="bg-slate-50 text-slate-500"><tr>{["Subscriber","Plan","Status","Amount","Billing","Start Date","End Date","Trial End","Created","Actions"].map(x=><th key={x} className="px-4 py-3 font-medium">{x}</th>)}</tr></thead><tbody className="divide-y">{subs.map(s=><tr key={s.id}><td className="px-4 py-3">{s.user_name||s.organization_name||"—"}<div className="text-xs text-slate-500">{s.user_email||s.organization_name}</div></td><td className="px-4 py-3">{s.plan_name||"Plan removed"}</td><td className="px-4 py-3 capitalize">{s.status.replace("_"," ")}</td><td className="px-4 py-3">{s.currency} {formatCurrencyAmount(s.amount,s.currency,language)}</td><td className="px-4 py-3 capitalize">{s.billing_period}</td><td className="px-4 py-3">{date(s.starts_at)}</td><td className="px-4 py-3">{date(s.ends_at)}</td><td className="px-4 py-3">{date(s.trial_ends_at)}</td><td className="px-4 py-3">{date(s.created_at)}</td><td className="px-4 py-3"><div className="flex gap-2"><button onClick={()=>setViewSub(s)} className="rounded border px-2 py-1">View</button><button onClick={()=>editSub(s)} className="rounded border px-2 py-1">Edit</button><select aria-label="Subscription status" value={s.status} onChange={e=>void subStatus(s,e.target.value)} className="rounded border px-2 py-1">{["trial","active","past_due","cancelled","expired"].map(v=><option key={v}>{v}</option>)}</select></div></td></tr>)}</tbody></table></div>}</div>}

 {globalPricePlan&&<GlobalPlanPrices plan={globalPricePlan} language={language} onClose={()=>setGlobalPricePlan(null)}/>}
 {planForm&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-3 sm:p-5"><form dir={language==="ar"?"rtl":"ltr"} onSubmit={savePlan} className="flex max-h-[calc(100dvh-1.5rem)] w-full max-w-3xl flex-col overflow-hidden rounded-2xl bg-white shadow-xl sm:max-h-[94dvh]"><div className="flex shrink-0 items-start justify-between gap-4 border-b px-5 py-4 sm:px-6"><div><h2 className="text-xl font-bold">{planEdit?t("Edit Plan"):t("Add Plan")}</h2><p className="mt-1 text-sm text-slate-500">Configure subscription pricing and access.</p></div><button type="button" onClick={()=>setPlanForm(null)} className="min-h-10 rounded-lg border px-3 py-1.5">Close</button></div><div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-5 py-5 sm:px-6"><section aria-labelledby="basic-details-heading"><h3 id="basic-details-heading" className="text-lg font-bold text-blue-950">{t("1. Basic Details")}</h3><div className="mt-3 grid gap-4 sm:grid-cols-2"><label className="text-sm">{t("Plan Name")} *<input required value={planForm.name} onChange={e=>setPlanForm({...planForm,name:e.target.value})} className={field}/></label><label className="text-sm">{t("Plan Code")} *<input required pattern="[a-zA-Z0-9][a-zA-Z0-9_-]{0,99}" value={planForm.code} onChange={e=>setPlanForm({...planForm,code:e.target.value.toLowerCase()})} className={field}/></label><label className="text-sm">{t("Price")} *<input required min="0" step="any" type="number" value={planForm.price} onChange={e=>setPlanForm({...planForm,price:Number(e.target.value)})} className={field}/></label><PlanCurrencySelect value={planForm.currency} existingCurrency={planEdit?originalPlanCurrency.current:undefined} onChange={currency=>setPlanForm({...planForm,currency})}/><label className="text-sm">{t("Billing Period")} *<select value={planForm.billing_period} onChange={e=>setPlanForm({...planForm,billing_period:e.target.value})} className={field}>{["monthly","yearly","lifetime","custom"].map(x=><option key={x}>{x}</option>)}</select></label><label className="text-sm">{t("Billing Type")}<select value={planForm.billing_type} onChange={e=>setPlanForm({...planForm,billing_type:e.target.value as "per_user"|"per_workspace"})} className={field}><option value="per_user">Per User</option><option value="per_workspace">Per Workspace</option></select></label><label className="text-sm">{t("Plan Visibility")}<select value={planForm.visibility} onChange={e=>setPlanForm({...planForm,visibility:e.target.value as "public"|"hidden"})} className={field}><option value="public">Public</option><option value="hidden">Hidden</option></select></label><label className="text-sm sm:col-span-2">{t("Description")}<textarea value={planForm.description} onChange={e=>setPlanForm({...planForm,description:e.target.value})} className={field}/></label></div></section><section className="mt-7" aria-labelledby="trial-config-heading"><h3 id="trial-config-heading" className="text-lg font-bold text-blue-950">{t("2. Free Trial Configuration")}</h3><div className="mt-3 grid gap-4 sm:grid-cols-2"><label className="text-sm">{t("Free Trial")}<select value={planForm.trial_mode} onChange={e=>setPlanForm({...planForm,trial_mode:e.target.value as typeof planForm.trial_mode,trial_days:e.target.value==="limited"?Math.max(1,planForm.trial_days):0})} className={field}><option value="disabled">{t("Disabled")}</option><option value="limited">{t("Limited Days")}</option><option value="unlimited">{t("Unlimited")}</option></select></label>{planForm.trial_mode==="limited"?<label className="text-sm">{t("Trial Days")}<div className="mt-1 flex items-center gap-2"><button type="button" aria-label="Decrease trial days" onClick={()=>setPlanForm({...planForm,trial_days:Math.max(1,planForm.trial_days-1)})} className="h-10 min-w-10 rounded-lg border">−</button><input required min="1" type="number" value={planForm.trial_days} onChange={e=>setPlanForm({...planForm,trial_days:Math.max(1,Number(e.target.value))})} className="h-10 w-24 rounded-lg border px-3 text-center"/><button type="button" aria-label="Increase trial days" onClick={()=>setPlanForm({...planForm,trial_days:planForm.trial_days+1})} className="h-10 min-w-10 rounded-lg border">+</button></div></label>:planForm.trial_mode==="unlimited"?<p className="self-end rounded-xl bg-blue-50 p-3 text-sm font-medium text-blue-800">{t("Unlimited Trial (does not expire)")}</p>:null}</div></section><PlanConfigurationSections limits={planForm.limits} features={planForm.features} setLimits={limits=>setPlanForm(current=>current?{...current,limits:limits as typeof current.limits}:current)} setFeatures={features=>setPlanForm(current=>current?{...current,features}:current)}/><section className="mt-7" aria-labelledby="plan-settings-heading"><h3 id="plan-settings-heading" className="text-lg font-bold text-blue-950">{t("5. Plan Settings")}</h3><div className="mt-3 grid gap-4 sm:grid-cols-2"><label className="text-sm">{t("Sort Order")}<input type="number" value={planForm.sort_order} onChange={e=>setPlanForm({...planForm,sort_order:Number(e.target.value)})} className={field}/></label><div className="flex flex-wrap items-center gap-5 pt-6 text-sm"><label className="flex items-center gap-2"><input type="checkbox" checked={planForm.is_featured} onChange={e=>setPlanForm({...planForm,is_featured:e.target.checked})}/> {t("Featured")}</label><label className="flex items-center gap-2"><input type="checkbox" checked={planForm.is_most_popular} onChange={e=>setPlanForm({...planForm,is_most_popular:e.target.checked})}/> {t("Most Popular")}</label><label className="flex items-center gap-2"><input type="checkbox" checked={planForm.is_active} onChange={e=>setPlanForm({...planForm,is_active:e.target.checked})}/> {t("Active")}</label></div></div></section></div><div className="z-10 flex shrink-0 justify-end gap-3 border-t bg-white px-5 py-4 sm:px-6"><button type="button" onClick={()=>setPlanForm(null)} className="min-h-11 rounded-xl border px-4 py-2">{t("Cancel")}</button><button disabled={saving} className="min-h-11 rounded-xl bg-blue-600 px-5 py-2 font-semibold text-white disabled:opacity-60">{saving?t("Saving…"):t("Save Plan")}</button></div></form></div>}{entitlementPlan&&<PlanEntitlementsEditor key={entitlementPlan.id} planId={entitlementPlan.id} planName={entitlementPlan.name} onClose={()=>setEntitlementPlan(null)} onSaved={(entitlements,registry)=>syncPlanEntitlements(entitlementPlan.id,entitlements,registry)}/>}{subForm&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4"><form onSubmit={saveSub} className="max-h-[94vh] w-full max-w-2xl overflow-auto rounded-2xl bg-white p-6 shadow-xl"><div className="flex justify-between"><h2 className="text-xl font-bold">{subEdit?"Edit Subscription":"Add Subscription"}</h2><button type="button" onClick={()=>setSubForm(null)} className="rounded-lg border px-3 py-1.5">Close</button></div><p className="mt-1 text-sm text-slate-500">Administrative record only. No payment is collected.</p><div className="mt-5 grid gap-4 sm:grid-cols-2"><label className="text-sm">Plan *<select required value={subForm.plan_id} onChange={e=>{const p=plans.find(x=>x.id===Number(e.target.value));setSubForm({...subForm,plan_id:e.target.value,amount:p?String(p.price):"",currency:p?.currency??"",billing_period:p?.billing_period??""})}} className={field}><option value="">Select plan</option>{plans.map(p=><option key={p.id} value={p.id}>{p.name} ({p.is_active?"active":"inactive"})</option>)}</select></label><label className="text-sm">Status<select value={subForm.status} onChange={e=>setSubForm({...subForm,status:e.target.value})} className={field}>{["trial","active","past_due","cancelled","expired"].map(v=><option key={v}>{v}</option>)}</select></label><label className="text-sm">User<select value={subForm.user_id} onChange={e=>setSubForm({...subForm,user_id:e.target.value,organization_id:e.target.value?"":subForm.organization_id})} className={field}><option value="">None</option>{users.map(u=><option key={u.id} value={u.id}>{u.name} · {u.email}</option>)}</select></label><label className="text-sm">Organization<select value={subForm.organization_id} onChange={e=>setSubForm({...subForm,organization_id:e.target.value,user_id:e.target.value?"":subForm.user_id})} className={field}><option value="">None</option>{orgs.map(o=><option key={o.id} value={o.id}>{o.name}</option>)}</select></label><label className="text-sm">Amount<input min="0" step="0.01" type="number" value={subForm.amount} onChange={e=>setSubForm({...subForm,amount:e.target.value})} className={field}/></label><label className="text-sm">Currency<input maxLength={3} value={subForm.currency} onChange={e=>setSubForm({...subForm,currency:e.target.value.toUpperCase()})} className={field}/></label><label className="text-sm">Billing Period<select value={subForm.billing_period} onChange={e=>setSubForm({...subForm,billing_period:e.target.value})} className={field}>{["monthly","yearly","lifetime","custom"].map(v=><option key={v}>{v}</option>)}</select></label>{([["starts_at","Start Date"],["ends_at","End Date"],["trial_ends_at","Trial End"]] as const).map(([key,label])=><label key={key} className="text-sm">{label}<input type="date" value={subForm[key]} onChange={e=>setSubForm({...subForm,[key]:e.target.value})} className={field}/></label>)}</div><div className="mt-6 flex justify-end gap-3"><button type="button" onClick={()=>setSubForm(null)} className="rounded-xl border px-4 py-2">{t("Cancel")}</button><button disabled={saving} className="rounded-xl bg-blue-600 px-4 py-2 font-semibold text-white">{saving?"Saving…":"Save Subscription"}</button></div></form></div>}
 {viewPlan&&(()=>{
  const yesNo=(value:boolean)=>t(value?"Enabled":"Disabled");
  const summary:[string,string][]=[["Plan Name",viewPlan.name],["Code",viewPlan.code],["Price",`${viewPlan.currency} ${formatCurrencyAmount(viewPlan.price,viewPlan.currency,language)}`],["Billing Period",t(viewPlan.billing_period)],["Trial Days",String(viewPlan.trial_days)],["Featured",yesNo(viewPlan.is_featured)],["Active",yesNo(viewPlan.is_active)],["Subscribers",String(viewPlan.subscriber_count)],["Created",date(viewPlan.created_at)],["Updated",date(viewPlan.updated_at)]];
  return <div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4"><div className="max-h-[90vh] w-full max-w-4xl overflow-auto rounded-2xl bg-white p-6"><div className="flex items-center justify-between gap-3"><h2 className="text-2xl font-bold">{t("Plan Details")}</h2><button onClick={()=>setViewPlan(null)} className="rounded border px-3 py-1">{t("Close")}</button></div>
   <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{summary.map(([label,value])=><div key={label} className="rounded-xl border bg-slate-50/70 p-3"><p className="text-xs font-medium text-slate-500">{t(label)}</p><p className="mt-1 break-words text-sm font-semibold text-slate-900">{value}</p></div>)}<div className="rounded-xl border bg-slate-50/70 p-3 sm:col-span-2 lg:col-span-3"><p className="text-xs font-medium text-slate-500">{t("Description")}</p><p className="mt-1 whitespace-pre-wrap text-sm text-slate-900">{viewPlan.description||t("—")}</p></div></div>
   <div className="mt-5 rounded-xl border p-4"><p className="text-sm text-slate-600">Entitlements are managed separately from pricing and subscription settings.</p><button onClick={()=>setEntitlementPlan(viewPlan)} className="mt-3 rounded-lg border px-3 py-2 text-sm font-semibold">Manage entitlements</button></div><h3 className="mt-6 font-semibold">{t("Subscribers")}</h3><div className="mt-2 divide-y rounded-xl border">{viewSubscribers.length===0?<p className="p-3 text-sm text-slate-500">{t("No subscribers.")}</p>:viewSubscribers.map(s=><p key={s.id} className="p-3 text-sm">{s.user_name||s.organization_name||t("Subscriber")} · {t(s.status)} · {s.currency} {formatCurrencyAmount(s.amount,s.currency,language)}</p>)}</div><button onClick={()=>{setViewPlan(null);selectTab("subscriptions");setSearch("")}} className="mt-3 rounded border px-3 py-2 text-sm">{t("View subscriptions")}</button></div></div>;
 })()}
 {viewSub&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4"><div className="w-full max-w-xl rounded-2xl bg-white p-6"><div className="flex justify-between"><h2 className="text-xl font-bold">Subscription #{viewSub.id}</h2><button onClick={()=>setViewSub(null)} className="rounded border px-3 py-1">Close</button></div><div className="mt-5 grid gap-3 sm:grid-cols-2">{[["Subscriber",viewSub.user_name||viewSub.organization_name||"—"],["Email",viewSub.user_email||"—"],["Organization",viewSub.organization_name||"—"],["Plan",viewSub.plan_name||"—"],["Status",viewSub.status],["Amount",`${viewSub.currency} ${viewSub.amount.toFixed(2)} (${viewSub.billing_period})`],["Start",date(viewSub.starts_at)],["End",date(viewSub.ends_at)],["Trial end",date(viewSub.trial_ends_at)],["Created",date(viewSub.created_at)]].map(([k,v])=><p key={k} className="text-sm"><span className="text-slate-500">{k}: </span>{v}</p>)}</div><button onClick={()=>{setViewSub(null);editSub(viewSub)}} className="mt-5 rounded-lg bg-blue-600 px-4 py-2 text-sm font-semibold text-white">Edit Subscription</button></div></div>}
 {viewPayment&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4" onMouseDown={e=>{if(e.target===e.currentTarget)setViewPayment(null)}}><section role="dialog" aria-modal="true" aria-labelledby="admin-payment-title" className="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl bg-white p-6 shadow-xl"><div className="flex items-center justify-between gap-4"><div><h2 id="admin-payment-title" className="text-2xl font-bold">Payment Details</h2><p className="mt-1 font-mono text-xs text-slate-500">{viewPayment.public_id}</p></div><button type="button" onClick={()=>setViewPayment(null)} className="rounded border px-3 py-2">Close</button></div><div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{[["Customer",viewPayment.subscriber_name],["Customer Email",viewPayment.subscriber_email],["Organization",viewPayment.organization_name||"—"],["Plan",viewPayment.plan_name],["Amount",`${viewPayment.currency} ${formatCurrencyAmount(viewPayment.amount,viewPayment.currency,language)}`],["Provider",viewPayment.provider],["Payment Method",viewPayment.payment_method||"—"],["Status",viewPayment.status],["Provider Order ID",viewPayment.provider_order_id||"—"],["Provider Payment ID",viewPayment.provider_payment_id||"—"],["Paid Date",date(viewPayment.paid_at)],["Created Date",date(viewPayment.created_at)],["Subscription",viewPayment.subscription?.status||"—"],["Subscription Start",date(viewPayment.subscription?.started_at||null)],["Subscription End",date(viewPayment.subscription?.ends_at||null)],["Payment Error Code",viewPayment.error_code||"—"],["Payment Error",viewPayment.error_description||"—"]].map(([label,value])=><div key={label} className="rounded-xl border bg-slate-50 p-3"><p className="text-xs text-slate-500">{label}</p><p className="mt-1 break-all text-sm font-semibold">{value}</p></div>)}</div><h3 className="mt-6 font-semibold">Payment Timeline</h3><div className="mt-2 divide-y rounded-xl border">{viewPayment.transactions?.length?viewPayment.transactions.map(x=><div key={x.id} className="flex flex-wrap justify-between gap-2 p-3 text-sm"><span><strong>{x.event_type}</strong><span className="ml-2 text-slate-500">{date(x.created_at)}</span></span><span>{x.currency} {formatCurrencyAmount(x.amount,x.currency,language)}</span></div>):<p className="p-3 text-sm text-slate-500">No payment events recorded.</p>}</div><div className="mt-5 flex flex-wrap gap-3"><button type="button" onClick={()=>printPayment(viewPayment)} className="rounded-lg border px-3 py-2">Print View</button>{["created","pending"].includes(viewPayment.status)&&<button type="button" disabled={cancellingPayment===viewPayment.id} onClick={()=>void cancelPayment(viewPayment)} className="rounded-lg border border-amber-300 px-3 py-2 text-amber-800 disabled:opacity-50">{cancellingPayment===viewPayment.id?"Cancelling…":"Cancel Payment"}</button>}{["failed","cancelled"].includes(viewPayment.status)&&<button type="button" disabled={deletingPayment===viewPayment.id} onClick={()=>void deletePayment(viewPayment)} className="rounded-lg border border-red-300 px-3 py-2 text-red-700 disabled:opacity-50">{deletingPayment===viewPayment.id?"Deleting…":"Delete Payment"}</button>}</div></section></div>}
 {viewTransaction&&(()=>{
  const payload=(viewTransaction.payload??{}) as Record<string,unknown>;

  const payloadString=(key:string)=>{
   const value=payload[key];
   return typeof value==="string"&&value.trim()?value.trim():"—";
  };

  const customerName=
   viewTransaction.subscriber_name==="Deleted User"
    ?"Former Customer"
    :(viewTransaction.subscriber_name||"—");

  const customerEmail=
   viewTransaction.subscriber_email?.endsWith("@deleted.invalid")
    ?"—"
    :(viewTransaction.subscriber_email||"—");

  const method=payloadString("method");

  const methodLabel=
   method==="—"
    ?"—"
    :method.charAt(0).toUpperCase()+method.slice(1);

  const receiptRows:[string,string][]=[
   ["Event ID",viewTransaction.provider_event_id||String(viewTransaction.id)],
   ["Payment Reference",viewTransaction.payment_public_id],
   ["Customer",customerName],
   ["Customer Email",customerEmail],
   ["Event Type",viewTransaction.event_type],
   ["Amount",`${viewTransaction.currency} ${formatCurrencyAmount(viewTransaction.amount,viewTransaction.currency,language)}`],
   ["Payment Status",viewTransaction.payment_status||viewTransaction.status||"—"],
   ["Payment Method",methodLabel],
   ["Provider Order ID",payloadString("order_id")],
   ["Provider Payment ID",payloadString("payment_id")],
   ["Bank",payloadString("bank")],
   ["Wallet",payloadString("wallet")],
   ["Created At",date(viewTransaction.created_at)]
  ];

  return <div
   className="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4"
   onMouseDown={e=>{
    if(e.target===e.currentTarget)setViewTransaction(null)
   }}
  >
   <section
    role="dialog"
    aria-modal="true"
    aria-labelledby="admin-transaction-title"
    className="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl bg-white p-6 shadow-xl"
   >
    <div className="flex items-start justify-between gap-4">
     <div>
      <p className="text-xs font-bold uppercase tracking-widest text-blue-700">
       Digital Card Platform
      </p>

      <h2
       id="admin-transaction-title"
       className="mt-1 text-2xl font-bold"
      >
       Payment Receipt
      </h2>

      <p className="mt-1 font-mono text-xs text-slate-500">
       {viewTransaction.provider_event_id||viewTransaction.id}
      </p>
     </div>

     <button
      type="button"
      onClick={()=>setViewTransaction(null)}
      className="rounded border px-3 py-2 print:hidden"
     >
      Close
     </button>
    </div>

    <div className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
     {receiptRows.map(([label,value])=>
      <div
       key={label}
       className="rounded-xl border bg-slate-50 p-3"
      >
       <p className="text-xs text-slate-500">{label}</p>
       <p className="mt-1 break-all text-sm font-semibold">{value}</p>
      </div>
     )}
    </div>

    <div className="mt-6 flex justify-end gap-3 print:hidden">
     <button
      type="button"
      onClick={()=>{
 const escapeHtml=(value:string)=>
  value
   .replaceAll("&","&amp;")
   .replaceAll("<","&lt;")
   .replaceAll(">","&gt;")
   .replaceAll('"',"&quot;")
   .replaceAll("'","&#039;");

 const rows=receiptRows.map(([label,value])=>`
  <div class="item">
   <div class="label">${escapeHtml(label)}</div>
   <div class="value">${escapeHtml(value)}</div>
  </div>
 `).join("");

 const iframe=document.createElement("iframe");
 iframe.setAttribute("aria-hidden","true");
 iframe.style.position="fixed";
 iframe.style.right="0";
 iframe.style.bottom="0";
 iframe.style.width="1px";
 iframe.style.height="1px";
 iframe.style.border="0";
 iframe.style.opacity="0";
 iframe.style.pointerEvents="none";

 document.body.appendChild(iframe);

 const frameWindow=iframe.contentWindow;
 const frameDocument=iframe.contentDocument;

 if(!frameWindow||!frameDocument){
  iframe.remove();
  setBillingError("Could not prepare receipt for printing.");
  return;
 }

 let cleaned=false;
 const cleanup=()=>{
  if(cleaned)return;
  cleaned=true;
  window.setTimeout(()=>{
   if(iframe.isConnected)iframe.remove();
  },300);
 };

 frameWindow.addEventListener("afterprint",cleanup,{once:true});

 frameDocument.open();
 frameDocument.write(`
  <!doctype html>
  <html>
   <head>
    <meta charset="utf-8" />
    <title>Payment Receipt</title>
    <style>
     @page {
      size: A4 portrait;
      margin: 10mm;
     }

     * {
      box-sizing: border-box;
     }

     html,
     body {
      margin: 0;
      padding: 0;
      width: 100%;
      height: auto;
      min-height: 0;
      overflow: visible;
      background: #fff;
      color: #000;
      font-family: Arial, Helvetica, sans-serif;
      -webkit-print-color-adjust: economy;
      print-color-adjust: economy;
     }

     .receipt {
      width: 100%;
      max-width: 720px;
      margin: 0 auto;
      padding: 0;
      break-after: avoid-page;
      page-break-after: avoid;
     }

     .brand {
      margin: 0 0 4px;
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 1px;
      text-transform: uppercase;
     }

     h1 {
      margin: 0;
      font-size: 22px;
      line-height: 1.2;
     }

     .reference {
      margin: 5px 0 14px;
      font-family: monospace;
      font-size: 10px;
      overflow-wrap: anywhere;
     }

     .grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 7px;
     }

     .item {
      border: 1px solid #000;
      border-radius: 5px;
      padding: 7px 8px;
      min-height: 46px;
      background: #fff;
      break-inside: avoid;
      page-break-inside: avoid;
     }

     .label {
      margin-bottom: 3px;
      font-size: 9px;
      line-height: 1.2;
     }

     .value {
      font-size: 11px;
      line-height: 1.25;
      font-weight: 700;
      overflow-wrap: anywhere;
     }

     .footer {
      margin-top: 12px;
      border-top: 1px solid #000;
      padding-top: 6px;
      text-align: center;
      font-size: 9px;
     }

     @media print {
      html,
      body,
      .receipt {
       width: 100%;
       height: auto !important;
       min-height: 0 !important;
       max-height: none !important;
       overflow: visible !important;
      }

      .receipt,
      .grid,
      .item {
       break-before: avoid-page !important;
       break-after: avoid-page !important;
      }
     }
    </style>
   </head>

   <body>
    <main class="receipt">
     <div class="brand">Digital Card Platform</div>
     <h1>Payment Receipt</h1>

     <div class="reference">
      ${escapeHtml(String(viewTransaction.provider_event_id||viewTransaction.id))}
     </div>

     <div class="grid">
      ${rows}
     </div>

     <div class="footer">
      Payment transaction receipt
     </div>
    </main>
   </body>
  </html>
 `);
 frameDocument.close();

 window.setTimeout(()=>{
  try{
   frameWindow.focus();
   frameWindow.print();
  }catch{
   setBillingError("Could not open the print dialog.");
   cleanup();
  }
 },250);

 window.setTimeout(cleanup,5000);
}}
      className="rounded-lg border px-4 py-2 font-semibold"
     >
      Print Receipt
     </button>

     <button
      type="button"
      onClick={()=>setViewTransaction(null)}
      className="rounded-lg bg-blue-600 px-4 py-2 font-semibold text-white"
     >
      Close
     </button>
    </div>
   </section>
  </div>;
 })()}
 
 </section>
}
