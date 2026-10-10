"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import { printPaymentReceipt } from "@/lib/customer-payment-print";
import { loadRazorpayCheckout } from "@/lib/razorpay-checkout";
import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { apiRequest } from "@/lib/user-api";
import { Summary, useUserData } from "./data";
import { LoadState } from "./Widgets";
import CurrencySelection, {useCurrencyChoice} from "@/components/pricing/CurrencySelection";
import {pricedPlans, checkoutMethodOptions, formatCurrencyAmount} from "@/lib/global-pricing";
import PlanPricingCard from "@/components/pricing/PlanPricingCard";
import { billingPeriod, formatPlanPrice, isFreePlan, maximumAnnualSavings, planChangeLabel, planFeatureLabels, planUsageLines, plansForBillingPeriod, PublicPlan } from "@/lib/pricing-plans";

type Payment = { payment_public_id:string; plan_id:number; plan_name:string; amount:number; currency:string; provider:string; provider_order_id:string|null; provider_payment_id:string|null; payment_method:string|null; status:string; paid_at:string|null; created_at:string };
type PaymentDetails = Payment & { customer_name:string; customer_email:string; subscription:{status:string;started_at:string|null;ends_at:string|null} };
type Subscription = { plan_id:number; plan_name:string; status:string; amount:number; currency:string; billing_period:string; starts_at:string; ends_at:string|null; trial_ends_at?:string|null; is_trial?:boolean };
type Plan = PublicPlan;
type Order = { payment_methods?:string[]; quote?:{subtotal:string;tax:string;total:string}; mode:"test"|"live"; key_id:string; razorpay_order_id:string; amount:number; amount_major:string; currency:string; billing_period:string; plan_name:string; prefill:{name:string;email:string;contact:string} };
type BillingProvider = {provider:"mock"|"razorpay"|"unavailable";mode?:"test"|"live";supported_currencies?:string[]};
type CheckoutResult = { razorpay_payment_id:string; razorpay_order_id:string; razorpay_signature:string };
declare global { interface Window { Razorpay?: new (options:Record<string,unknown>)=>{open:()=>void;on:(event:string, cb:(value:unknown)=>void)=>void} } }
function humanizeFeatures(t:(key:string,params?:Record<string,string|number>)=>string,features?:string[]|null,limits?:Record<string,unknown>|null) {
 return planFeatureLabels({id:0,name:"",description:"",price:0,currency:"USD",billing_period:"monthly",trial_days:0,feature_limits:{features:features||[],limits:limits||{}}},t);
}
function PlanFeatures({items}:{items:string[]}) {
 const { t } = useCustomerLocale();
 
 return items.length?<ul className="grid gap-2 sm:grid-cols-2">{items.map((item,i)=><li key={`${item}-${i}`} className="flex items-start gap-2 text-sm text-slate-700"><span aria-hidden="true" className="font-bold text-emerald-600">✓</span><span>{t(item)}</span></li>)}</ul>:<p className="text-sm text-slate-500">{t("No feature details are available for this plan.")}</p>; }

export default function Billing({planID,billingPeriod:requestedBillingPeriod,currency,country,reviewedAmount}:{planID?:string;billingPeriod?:string;currency?:string;country?:string;reviewedAmount?:string}) {
 const {choice,update}=useCurrencyChoice(currency?{currency,country:country||""}:undefined);
 const { t, error: localizeError, language } = useCustomerLocale();
 const customerDate=(value:string|null)=>value?new Date(value).toLocaleDateString(language === "ar" ? "ar" : "en",{year:"numeric",month:"short",day:"numeric"}):"—";

 const router=useRouter();
 const summary=useUserData<Summary>("/user/dashboard");
 const history=useUserData<{payments:Payment[];subscriptions:Subscription[];current_subscription:Subscription|null}> ("/billing/history");
 const plans=useUserData<{data:Plan[]}> (`/website/pricing?lang=${language}`);
 const provider=useUserData<BillingProvider>("/billing/provider");
 const [busy,setBusy]=useState(false); const [message,setMessage]=useState(""); const [failure,setFailure]=useState(""); const [selected,setSelected]=useState<Plan|null>(null); const [viewingPayment,setViewingPayment]=useState<PaymentDetails|null>(null); const [receiptView,setReceiptView]=useState(false); const [paymentLoading,setPaymentLoading]=useState(false); const [paymentError,setPaymentError]=useState("");
 const [printing,setPrinting]=useState(false);
 const [selectedPeriod,setSelectedPeriod]=useState<"monthly"|"yearly">(requestedBillingPeriod==="yearly"?"yearly":"monthly");
 const [printError,setPrintError]=useState(false);
 const printBusy=useRef(false);
 const [payNowConsent,setPayNowConsent]=useState("");
 const consentKey=(plan:Plan)=>`${plan.id}:${plan.price}:${plan.currency}:${plan.billing_period}`;
 const hasTrial=(plan:Plan)=>!isFreePlan(plan)&&(plan.trial_mode==="unlimited"||(plan.trial_mode==="limited"&&plan.trial_days>0));
 const trialUsed=history.data?.subscriptions?.some(s=>s.is_trial||(s.is_trial===undefined&&(s.status==="trial"||s.trial_ends_at)))||false;
 async function startTrial(plan:Plan){
  if(checkoutBusy.current)return;checkoutBusy.current=true;setBusy(true);setFailure("");setMessage("");
  try{await apiRequest("/billing/free/activate",{method:"POST",body:JSON.stringify({plan_id:plan.id})});setSelected(null);setMessage("Your free trial is active. No payment has been taken. Return to checkout when you are ready to purchase.");refreshBilling();}
  catch(e){setFailure(e instanceof Error?e.message:"Could not start your trial. Please retry.");}
  finally{checkoutBusy.current=false;setBusy(false);}
 }
 const paymentFields = viewingPayment ? [["Customer Name",viewingPayment.customer_name||"—"],["Customer Email",viewingPayment.customer_email||"—"],["Payment Reference",viewingPayment.payment_public_id],["Plan",viewingPayment.plan_name],["Amount",`${viewingPayment.currency} ${formatCurrencyAmount(viewingPayment.amount,viewingPayment.currency,language)}`],["Provider",t(viewingPayment.provider)],["Payment Method",t(viewingPayment.payment_method||"—")],["Status",t(viewingPayment.status)],["Provider Order ID",viewingPayment.provider_order_id||"—"],["Provider Payment ID",viewingPayment.provider_payment_id||"—"],["Paid Date",customerDate(viewingPayment.paid_at)],["Created Date",customerDate(viewingPayment.created_at)],["Subscription",t(viewingPayment.subscription?.status||"—")],["Subscription Period",viewingPayment.subscription?.started_at?`${customerDate(viewingPayment.subscription.started_at)} – ${customerDate(viewingPayment.subscription.ends_at)}`:"—"]] : [];
 async function printCurrentPayment() {
  if (!viewingPayment || printBusy.current) return;
  printBusy.current=true;setPrinting(true);setPrintError(false);
  try {
   await printPaymentReceipt({
    branding:t("Digital Card Platform"), heading:t("Payment Details"), language,
    fields:paymentFields.map(([label,value])=>({label:t(label),value,direction:["Customer Email","Payment Reference","Provider Order ID","Provider Payment ID"].includes(label)?"ltr":"auto"})),
   });
  } catch { setPrintError(true); }
  finally { printBusy.current=false;setPrinting(false); }
 }
 const checkoutBusy=useRef(false);
 const handledSelection=useRef("");
 const current=history.data?.current_subscription;
 useEffect(()=>{const fallback=Boolean(plans.data?.data.some(plan=>plan._locale_fallback));window.dispatchEvent(new CustomEvent("website-locale-fallback",{detail:{section:"billing-plans",fallback:language==="ar"&&fallback}}));},[plans.data,language]);
 const availablePlans=useMemo(()=>pricedPlans(plans.data?.data||[],choice.currency,choice.country),[plans.data,choice]);
 const currentPlan=availablePlans.find(p=>p.id===current?.plan_id);
 const currentPlanId=current&&["active","trial","trialing","past_due"].includes(current.status)?current.plan_id:null;

 const hasMonthly=availablePlans.some(plan=>billingPeriod(plan)==="monthly");
 const hasYearly=availablePlans.some(plan=>billingPeriod(plan)==="yearly");
 const visiblePlans=plansForBillingPeriod(availablePlans,selectedPeriod);
 const recommendedPlanId=visiblePlans.find(plan=>plan.is_most_popular===true&&Number(plan.price)>0&&plan.pricing_type!=="custom"&&plan.billing_period.toLowerCase()!=="custom")?.id;
 const annualSavings=maximumAnnualSavings(availablePlans.filter(plan=>!plan.pricing_unavailable));
 useEffect(()=>{
  if(!planID||!plans.data)return;
  const selection=`${planID}:${requestedBillingPeriod||""}:${choice.currency}:${choice.country}`;
  if(handledSelection.current===selection)return;
  handledSelection.current=selection;
  const requested=availablePlans.find(plan=>String(plan.id)===planID&&(!requestedBillingPeriod||plan.billing_period===requestedBillingPeriod));
  if(!requested){setFailure(availablePlans.some(p=>String(p.id)===planID)?"The selected billing period does not match this plan. Please select the plan again.":"The selected plan is no longer available. Please choose an available plan.");return;}
  if(requestedBillingPeriod&&requestedBillingPeriod.toLowerCase()!==requested.billing_period.toLowerCase()){
   setFailure("The selected billing period does not match this plan. Please select the plan again.");return;
  }
  setSelected(requested);
 },[plans.data,planID,requestedBillingPeriod,availablePlans,choice]);
 useEffect(()=>{
  const rows=availablePlans;
  const wanted=requestedBillingPeriod==="yearly"?"yearly":requestedBillingPeriod==="monthly"?"monthly":null;
  const requestedPlan=rows.find(plan=>String(plan.id)===planID);
  const wantedPlanPeriod=requestedPlan?billingPeriod(requestedPlan):"other";
  setSelectedPeriod(currentPeriod=>{
   if(wanted&&(wanted==="monthly"?hasMonthly:hasYearly))return wanted;
   if((wantedPlanPeriod==="monthly"&&hasMonthly)||(wantedPlanPeriod==="yearly"&&hasYearly))return wantedPlanPeriod;
   if((currentPeriod==="monthly"&&hasMonthly)||(currentPeriod==="yearly"&&hasYearly))return currentPeriod;
   return hasMonthly?"monthly":"yearly";
  });
 },[plans.data,planID,requestedBillingPeriod,hasMonthly,hasYearly,availablePlans]);
 function refreshBilling(){summary.reload();history.reload();plans.reload();}
 async function viewPayment(payment:Payment,receipt=false){setPaymentLoading(true);setPaymentError("");setPrintError(false);setReceiptView(receipt);try{const result=await apiRequest(`/billing/history/${encodeURIComponent(payment.payment_public_id)}`);setViewingPayment(result.data as PaymentDetails);}catch(e){setPaymentError(e instanceof Error?e.message:"Could not load payment details.");}finally{setPaymentLoading(false);}}
 async function checkout(planId:number) {
  if(checkoutBusy.current) return;
  if(selected&&isCurrent(selected)){setFailure("You already have this plan.");return;}
  checkoutBusy.current=true;setBusy(true);setMessage("");setFailure("");
  const release=()=>{checkoutBusy.current=false;setBusy(false);};
  try {
   const selectedPlan=availablePlans.find(p=>p.id===planId&&p.billing_period===(selected?.billing_period||selectedPeriod));
   if(!selectedPlan)throw new Error("The selected plan is no longer available. Please choose an available plan.");
   if(selectedPlan.pricing_unavailable)throw new Error(language==="ar"?"لا يوجد سعر لهذه العملة والفترة. اختر عملة أخرى.":"No price is configured for this currency and period. Select another currency.");
   if(selectedPlan?.pricing_type==="custom"||selectedPlan?.billing_period==="custom"){router.push("/contact?purpose=enterprise");release();return;}
   if(isFreePlan(selectedPlan)){
    await apiRequest("/billing/free/activate",{method:"POST",body:JSON.stringify({plan_id:planId})});
    setMessage("Your free plan is active.");setSelected(null);refreshBilling();release();return;
   }
   const confirmedPlan=selected?.id===planId?selected:selectedPlan;
   if(hasTrial(selectedPlan)&&payNowConsent!==consentKey(confirmedPlan))throw new Error("Please choose a free trial or agree to pay now before continuing to checkout.");
   // Recheck on click so a stale provider response cannot route a real checkout
   // into the demo page (or make the UI state authoritative).
   const selectedProvider=await apiRequest("/billing/provider") as BillingProvider;
   if(selectedProvider.provider==="mock") throw new Error("Verified payment checkout is unavailable while Demo Payment is configured. Please contact support to enable the payment provider.");
   if(selectedProvider.provider!=="razorpay") throw new Error("Checkout is not configured for this environment.");
   if(reviewedAmount&&handledSelection.current&&selected?.id===planId&&Number(reviewedAmount)!==Number(selected.price)&&payNowConsent!==consentKey(selected)) {
    setPayNowConsent(consentKey(selected));
    throw new Error(language==="ar"?"تغير السعر. راجع المبلغ ثم أكد الدفع مرة أخرى.":"The plan total changed. Review the updated amount and confirm again.");
   }
   const order:Order=await apiRequest("/billing/checkout/order",{method:"POST",body:JSON.stringify({plan_id:planId,billing_period:selectedPlan.billing_period.toLowerCase(),currency:confirmedPlan.currency,country:choice.country,expected_amount:confirmedPlan.quote_total||String(confirmedPlan.price),pay_now:hasTrial(selectedPlan)?true:undefined})});
   if(!order.amount_major||!Number.isFinite(Number(order.amount_major))||Number(order.amount_major)<=0||!order.billing_period) throw new Error("Unable to confirm the checkout price. Please refresh billing or contact support.");
   if(order.billing_period!==confirmedPlan.billing_period.toLowerCase()) throw new Error("The selected billing period does not match this plan. Please select the plan again.");
   if(order.currency.toUpperCase()!==confirmedPlan.currency.toUpperCase())throw new Error("The payment provider returned a different currency. Refresh billing or contact support.");
   if(Number(order.amount_major)!==Number(confirmedPlan.price)) {
    setSelected({...selectedPlan,price:Number(order.amount_major),currency:order.currency,billing_period:order.billing_period,quote_total:order.amount_major,subtotal:order.quote?.subtotal||order.amount_major,tax:order.quote?.tax||"0"});
    throw new Error("The plan total changed. Review the updated amount and confirm again.");
   }
   await loadRazorpayCheckout();
   if(!window.Razorpay) throw new Error("Unable to load secure checkout. Please try again.");
   let verifying=false;
   let failed=false;
   const checkout=new window.Razorpay({
    key:order.key_id,order_id:order.razorpay_order_id,amount:order.amount,currency:order.currency,
    name:"Digital Card Platform",description:order.plan_name,prefill:order.prefill,method:checkoutMethodOptions(order.payment_methods),
    modal:{ondismiss:()=>{if(!verifying){if(!failed)setMessage("Checkout closed. No subscription change has been made. You can retry when ready.");release();refreshBilling();}}},
    handler:async (result:unknown)=>{
     verifying=true;setFailure("");setMessage("Confirming your payment…");
     const payment=result as CheckoutResult;
     try {
      const verified=await apiRequest("/billing/checkout/verify",{method:"POST",body:JSON.stringify({razorpay_order_id:payment.razorpay_order_id,razorpay_payment_id:payment.razorpay_payment_id,razorpay_signature:payment.razorpay_signature})});
      if(verified.status==="paid") {setMessage("Payment successful. Your plan has been activated.");setSelected(null);}
      else setMessage("Payment is awaiting confirmation. Your subscription will update after confirmation; refresh billing shortly.");
     } catch {setMessage("");setFailure("We could not confirm your payment yet. Please refresh billing before trying again. A captured payment can also be confirmed by our secure webhook.");}
     finally {refreshBilling();release();}
    }
   });
   checkout.on("payment.failed",()=>{failed=true;setMessage("");setFailure("Payment failed. Your subscription has not changed. You can retry in checkout or close it and try again.");history.reload();});
   checkout.open();
   setSelected(null);
  } catch(e){setFailure(e instanceof Error?e.message:"Unable to start payment. Please try again.");handledSelection.current="";plans.reload();release();}
 }
 function currencyUnavailable(p:Plan){return !isFreePlan(p)&&p.pricing_type!=="custom"&&p.billing_period!=="custom"&&Number(p.price)>0&&provider.data?.provider!=="unavailable"&&!!provider.data&&!provider.data.supported_currencies?.includes(p.currency.toUpperCase());}
 function purchaseUnavailable(p:Plan){return currencyUnavailable(p)||(!isFreePlan(p)&&p.pricing_type!=="custom"&&(!provider.data||provider.data.provider==="unavailable"));}
 function isCurrent(p:Plan){return currentPlanId===p.id&&(!current?.billing_period||current.billing_period===p.billing_period)&&(!current?.currency||current.currency===p.currency)&&!((current?.status==="trial"||current?.status==="trialing")&&!isFreePlan(p));}
 const currencyLabel=(currency:string,price:number)=>new Intl.NumberFormat(language === "ar" ? "ar" : "en",{style:"currency",currency,currencyDisplay:"code"}).format(Number(price));
 const currentFeatures=humanizeFeatures(t,currentPlan?.feature_limits?.features||summary.data?.current_plan?.features,currentPlan?.feature_limits?.limits||summary.data?.current_plan?.limits);
 return <div className="space-y-8">{provider.data?.provider==="razorpay"&&provider.data.mode==="test"&&<p className="rounded-xl bg-amber-50 p-4 text-sm font-semibold text-amber-900">{t("TEST MODE — Razorpay test payments only.")}</p>}<div><h1 className="text-3xl font-black text-slate-950">{t("Plans & Billing")}</h1><p className="mt-2 text-slate-600">{t("Manage your subscription and payment history.")}</p></div>
 {provider.data?.provider==="mock"&&<p className="rounded-xl border border-blue-200 bg-blue-50 p-4 text-sm font-medium text-blue-900">{t("Demo Payment is enabled for this local environment. No real money will be charged.")}</p>}{provider.error&&<p role="alert" className="rounded-xl bg-red-50 p-4 text-sm text-red-800">{t("Could not load payment options:")}{localizeError(provider.error)}</p>}{provider.data?.provider==="unavailable"&&<p role="alert" className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900">{t("Checkout is unavailable in this environment.")}</p>}
 {message&&<><p role="status" className="rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-emerald-800">{t(message)}</p><button type="button" onClick={refreshBilling} className="rounded-xl border border-violet-200 bg-white px-4 py-2 font-semibold text-violet-700">{t("Refresh Subscription")}</button></>}{failure&&<p role="alert" className="rounded-xl border border-red-200 bg-red-50 p-4 text-red-800">{localizeError(failure)}</p>}
 {busy&&<p role="status" className="rounded-xl bg-violet-50 p-4 text-violet-800">{t("Preparing secure checkout…")}</p>}
 
 <LoadState {...summary} retry={summary.reload}/>
 {history.data && <section className="rounded-2xl border border-slate-200 bg-white p-6"><div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="text-xl font-bold">{t("Current Plan")}</h2>{current&&<p className="mt-1 text-sm text-slate-500">{t("Your active subscription and included features")}</p>}</div>{current&&<span className="rounded-full bg-emerald-50 px-3 py-1 text-sm font-semibold capitalize text-emerald-700">{t(current.status)}</span>}</div>{current ? <><div className="mt-4 flex flex-wrap items-baseline gap-3"><span className="text-2xl font-black text-slate-950">{current.plan_name}</span>{current.amount!=null&&current.currency&&<span className="font-semibold text-slate-700">{currencyLabel(current.currency,current.amount)} / {t(current.billing_period)}</span>}</div><dl className="mt-5 grid gap-4 text-sm sm:grid-cols-3"><div><dt className="text-slate-500">{t("Billing period")}</dt><dd className="mt-1 font-semibold capitalize">{t(current.billing_period)}</dd></div><div><dt className="text-slate-500">{t("Start date")}</dt><dd className="mt-1 font-semibold">{customerDate(current.starts_at)}</dd></div><div><dt className="text-slate-500">{t("Renewal / end date")}</dt><dd className="mt-1 font-semibold">{customerDate(current.status==="trial"?current.trial_ends_at||null:current.ends_at)}</dd></div></dl><h3 className="mt-6 mb-3 font-semibold">{t("Included features")}</h3><PlanFeatures items={currentFeatures}/></> : <p className="mt-3 text-slate-600">{t("No active subscription yet. Choose a plan below to get started.")}</p>}</section>}
 <CurrencySelection choice={choice} language={language} onChange={next=>{setSelected(null);setPayNowConsent("");handledSelection.current="";update(next);}}/>
 <section>
  <div className="mb-4 flex flex-wrap items-center justify-between gap-4"><div><h2 className="text-2xl font-bold">{t("Choose the plan that fits you")}</h2><p className="mt-1 text-slate-600">{t("Simple, flexible pricing for everyone.")}</p></div>
   <div role="group" aria-label={t("Billing period")} className="inline-flex rounded-full border border-violet-200 bg-white p-1 shadow-sm">
    <button type="button" aria-pressed={selectedPeriod==="monthly"} disabled={!hasMonthly} onClick={()=>setSelectedPeriod("monthly")} className={`min-h-11 rounded-full px-5 text-sm font-bold transition ${selectedPeriod==="monthly"?"bg-violet-600 text-white shadow":"text-slate-600 hover:bg-violet-50"} disabled:cursor-not-allowed disabled:opacity-45`}>{t("Monthly")}</button>
    <button type="button" aria-pressed={selectedPeriod==="yearly"} disabled={!hasYearly} onClick={()=>setSelectedPeriod("yearly")} className={`flex min-h-11 items-center gap-2 rounded-full px-5 text-sm font-bold transition ${selectedPeriod==="yearly"?"bg-violet-600 text-white shadow":"text-slate-600 hover:bg-violet-50"} disabled:cursor-not-allowed disabled:opacity-45`}>{t("Yearly")}{annualSavings>0&&<span className={`rounded-full px-2 py-0.5 text-[10px] font-extrabold ${selectedPeriod==="yearly"?"bg-white/20 text-white":"bg-emerald-50 text-emerald-700"}`}>{t("Save up to {count}%",{count:annualSavings})}</span>}</button>
   </div>
  </div>
  <LoadState {...plans} retry={plans.reload}/>
  {visiblePlans.length>0&&<div className="grid auto-rows-fr gap-5 sm:grid-cols-2 lg:grid-cols-3">{visiblePlans.map(p=>{
   const custom=p.pricing_type==="custom"||p.billing_period.toLowerCase()==="custom";
   const free=isFreePlan(p);
   const actionLabel=isCurrent(p)?t("Current Plan"):custom?t(p.cta_text||"Contact Sales"):currentPlanId===p.id&&!free?t("Upgrade Plan"):currentPlan?t(planChangeLabel(p,currentPlan)):free?t("Get Started"):t("Upgrade Plan");
   const blocked=purchaseUnavailable(p);
   const disabledReason=currencyUnavailable(p)?t("This currency is not enabled for the configured payment provider."):blocked?t("Checkout is not configured for this plan."):undefined;
   return <PlanPricingCard key={p.id+":"+p.billing_period} plan={p} features={planFeatureLabels(p,t)} price={formatPlanPrice(p,t,language)} popular={p.id===recommendedPlanId} current={isCurrent(p)} actionLabel={actionLabel} disabled={busy} disabledReason={disabledReason} onAction={()=>{setFailure("");setPayNowConsent("");setSelected(p);}} onViewDetails={()=>{setFailure("");setPayNowConsent("");setSelected(p);}} t={t} language={language}/>;
  })}</div>}
  {!plans.loading&&plans.data?.data.length===0&&<p className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center text-slate-600">{t("No plans are available currently.")}</p>}
  {plans.data?.data.length&&visiblePlans.length===0&&<p className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center text-slate-600">{t("No {period} plans are available yet.",{period:t(selectedPeriod)})}</p>}
 </section>
 <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white"><div className="p-6"><h2 className="text-xl font-bold">{t("Payment / Billing History")}</h2></div><LoadState {...history} retry={history.reload}/>{history.data&&(history.data.payments.length?<div className="overflow-x-auto"><table className="w-full min-w-[1050px] text-start text-sm"><thead className="bg-slate-50 text-slate-500"><tr>{["Plan Name","Payment Reference","Provider","Amount","Currency","Payment Method","Status","Paid Date","Created Date",""].map(x=><th key={x} className="p-4">{t(x)}</th>)}</tr></thead><tbody>{history.data.payments.map(p=><tr className="border-t border-slate-100" key={p.payment_public_id}><td className="p-4">{p.plan_name}{p.provider==="mock"&&<span className="ms-2 rounded-full bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{t("Demo")}</span>}</td><td className="p-4 font-mono text-xs" title={p.payment_public_id}><bdi dir="ltr">{p.payment_public_id}</bdi></td><td className="p-4 capitalize">{t(p.provider)}</td><td className="p-4">{formatCurrencyAmount(p.amount,p.currency,language)}</td><td className="p-4"><bdi dir="ltr">{p.currency}</bdi></td><td className="p-4 capitalize">{t(p.payment_method||"—")}</td><td className="p-4 capitalize">{t(p.status)}</td><td className="p-4">{customerDate(p.paid_at)}</td><td className="p-4">{customerDate(p.created_at)}</td><td className="p-4"><div className="flex flex-wrap gap-3"><button type="button" onClick={()=>void viewPayment(p)} className="whitespace-nowrap font-semibold text-blue-700">{t("View Details")}</button><button type="button" onClick={()=>void viewPayment(p,true)} className="whitespace-nowrap font-semibold text-blue-700">{t("View Receipt")}</button>{p.status==="failed"&&<button disabled={busy||currentPlanId===p.plan_id} onClick={()=>checkout(p.plan_id)} className="font-semibold text-slate-600 disabled:opacity-50">{t("Retry")}</button>}</div></td></tr>)}</tbody></table></div>:<p className="px-6 pb-8 text-slate-500">{t("No payments yet.")}</p>)}</section>
 {paymentLoading&&<div role="status" className="rounded-xl border bg-white p-5 text-sm text-slate-600">{t("Loading payment details…")}</div>}{paymentError&&<div role="alert" className="rounded-xl bg-red-50 p-4 text-sm text-red-800">{localizeError(paymentError)}</div>}
 {viewingPayment&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-950/50 p-4" onMouseDown={e=>{if(e.target===e.currentTarget)setViewingPayment(null);}}><section role="dialog" aria-modal="true" aria-labelledby="customer-payment-title" className="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl bg-white p-6 shadow-xl"><div className="flex items-center justify-between gap-4"><div><p className="text-xs font-bold uppercase tracking-widest text-blue-700">{t("Digital Card Platform")}</p><h2 id="customer-payment-title" className="mt-1 text-2xl font-black">{receiptView?t("Payment Receipt"):t("Payment Details")}</h2></div><button type="button" onClick={()=>setViewingPayment(null)} className="rounded-lg border px-3 py-2">{t("Close")}</button></div><div className="mt-5 grid gap-4 sm:grid-cols-2">{paymentFields.map(([label,value])=><div key={label} className="rounded-xl border bg-slate-50 p-3"><p className="text-xs text-slate-500">{t(label)}</p><p className="mt-1 break-all text-sm font-semibold"><bdi dir={["Customer Email","Payment Reference","Provider Order ID","Provider Payment ID"].includes(label)?"ltr":"auto"}>{value}</bdi></p></div>)}</div>{printError&&<p role="alert" className="mt-4 text-sm text-red-700">{t("Something went wrong. Please try again.")}</p>}<button type="button" disabled={printing} onClick={()=>void printCurrentPayment()} className="mt-6 rounded-xl bg-blue-600 px-4 py-2 font-semibold text-white disabled:opacity-50">{printing?t("Loading…"):t("Print View")}</button></section></div>}
 {selected&&<div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/50 p-4" onMouseDown={e=>{if(e.target===e.currentTarget)setSelected(null);}}><section role="dialog" aria-modal="true" aria-labelledby="plan-dialog-title" className="max-h-[90vh] w-full max-w-xl overflow-y-auto rounded-3xl bg-white p-6 shadow-2xl sm:p-8"><div className="flex items-start justify-between gap-4"><div><h2 id="plan-dialog-title" className="text-2xl font-black text-slate-950">{selected.name}</h2>{selected.is_most_popular&&<span className="mt-2 inline-block rounded-full bg-violet-100 px-3 py-1 text-xs font-semibold text-violet-700">{t("Most Popular")}</span>}</div><button type="button" aria-label={t("Close plan details")} onClick={()=>setSelected(null)} className="rounded-lg p-2 text-xl leading-none text-slate-500 hover:bg-slate-100">×</button></div><p className="mt-5 text-3xl font-black text-slate-950">{formatPlanPrice(selected,t,language).amount}{formatPlanPrice(selected,t,language).period&&<span className="ms-2 text-base font-medium text-slate-500">{formatPlanPrice(selected,t,language).period}</span>}</p><p className="mt-1 text-sm font-semibold text-slate-700">{t("Amount due now")}: {selected.pricing_unavailable?formatPlanPrice(selected,t,language).amount:currencyLabel(selected.currency,selected.price)} / {t(selected.billing_period)}</p>{selected?.quote_total&&<div className="rounded-xl border bg-white p-4 text-sm"><p>{language==="ar"?"المجموع الفرعي":"Subtotal"}: {selected.currency} {selected.subtotal}</p><p>{language==="ar"?"الضريبة":"Tax"}: {selected.currency} {selected.tax}</p><p className="font-bold">{language==="ar"?"الإجمالي":"Total"}: {selected.currency} {selected.quote_total}</p></div>}<p className="mt-4 whitespace-pre-line text-slate-600">{selected.description||t("Plan details and feature limits are shown below.")}</p><p className="mt-3 text-sm text-slate-600">{t("Trial availability")} <strong>{selected.trial_mode==="unlimited"?t("Unlimited Trial"):selected.trial_mode==="limited"&&selected.trial_days>0?t("{count} day trial",{count:selected.trial_days}):t("No trial")}</strong></p><p className="mt-2 text-sm text-slate-600"><strong>{t("Subscription terms")}:</strong> {selected.billing_period.toLowerCase()==="lifetime"?t("One-time payment"):t("Payment covers {period}. Renewal requires another payment.",{period:t(selected.billing_period)})}</p><p className="mt-2 text-sm text-slate-600"><strong>{t("Billing type")}:</strong> {t(selected.billing_type==="per_workspace"?"Per Workspace":"Per User")}</p><h3 className="mb-3 mt-6 font-bold">{t("Included features and limits")}</h3><PlanFeatures items={planFeatureLabels(selected,t)}/>{planUsageLines(selected,t).length>0&&<><h3 className="mb-3 mt-6 font-bold">{t("Usage limits")}</h3><PlanFeatures items={planUsageLines(selected,t)}/></>}<p className="mt-4 text-sm font-semibold">{t("Billing period")}: {t(selected.billing_period)}</p>{hasTrial(selected)&&<div className="mt-4 space-y-3 rounded-xl border border-violet-200 p-4"><p className="text-sm">{t("A free trial takes no payment. It ends without an automatic charge; return to checkout to purchase.")}</p>{!trialUsed&&<button type="button" disabled={busy} onClick={()=>void startTrial(selected)} className="rounded-xl border border-violet-300 px-4 py-2 font-semibold text-violet-700">{t("Start Free Trial")}</button>}{trialUsed&&<p className="text-sm">{t("You have already used a free trial. Please choose paid checkout.")}</p>}<label className="flex items-start gap-2 text-sm"><input type="checkbox" checked={payNowConsent===consentKey(selected)} onChange={e=>setPayNowConsent(e.target.checked?consentKey(selected):"")} /><span>{t("I agree to pay the displayed amount now and end any remaining trial.")}</span></label></div>}{failure&&<p role="alert" className="mt-4 rounded-xl bg-red-50 p-4 text-red-800">{localizeError(failure)}</p>}<div className="mt-8 grid gap-3 sm:grid-cols-2"><button type="button" onClick={()=>setSelected(null)} className="rounded-xl border border-slate-300 px-4 py-3 font-semibold text-slate-700 hover:bg-slate-50">{t("Close")}</button><button type="button" disabled={isCurrent(selected)||busy||selected.pricing_unavailable} onClick={()=>void checkout(selected.id)} className="rounded-xl bg-violet-600 px-4 py-3 font-bold text-white hover:bg-violet-700 disabled:cursor-not-allowed disabled:bg-slate-300">{busy?t("Preparing secure checkout…"):isCurrent(selected)?t("Current Plan"):selected.pricing_type==="custom"?t(selected.cta_text||"Contact Sales"):isFreePlan(selected)?t("Get Started"):t("Continue to checkout")}</button></div></section></div>}
 </div>;
}

