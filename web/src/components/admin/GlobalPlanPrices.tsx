"use client";
import {useEffect,useState} from "react";
import {adminApi} from "@/lib/admin-api";
import type {PriceConfig,PriceRule} from "@/lib/global-pricing";
const field="rounded-lg border px-3 py-2 text-sm";
export default function GlobalPlanPrices({plan,onClose,language}:{plan:{id:number;name:string;currency:string};onClose:()=>void;language:string}){
 const [config,setConfig]=useState<PriceConfig|null>(null),[error,setError]=useState(""),[saving,setSaving]=useState(false),[notice,setNotice]=useState("");
 const [currencies,setCurrencies]=useState<{code:string;exponent:number}[]>([]);
 const ar=language==="ar",text=(en:string,arabic:string)=>ar?arabic:en;
 useEffect(()=>{let live=true;Promise.all([adminApi("/admin/plans/"+plan.id+"/billing-prices"),adminApi("/admin/settings/payment-currencies")]).then(([p,c])=>{if(live){setConfig(p.data);setCurrencies(c.data.currencies)}}).catch(e=>{if(live)setError(e.message)});return()=>{live=false}},[plan.id]);
 function update(index:number,patch:Partial<PriceRule>){setConfig(c=>c?{...c,prices:c.prices.map((r,i)=>i===index?{...r,...patch}:r)}:c);setNotice("");}
 async function save(){if(!config)return;setSaving(true);setError("");try{const response=await adminApi("/admin/plans/"+plan.id+"/billing-prices",{method:"PUT",body:JSON.stringify(config)});setConfig(response.data);setNotice(text("Prices saved.","تم حفظ الأسعار."));}catch(e){setError(e instanceof Error?e.message:"Could not save prices")}finally{setSaving(false)}}
 return <div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/50 p-4"><section dir={ar?"rtl":"ltr"} className="max-h-[90vh] w-full max-w-5xl overflow-auto rounded-2xl bg-white p-6"><div className="flex justify-between gap-4"><h2 className="text-xl font-bold">{text("Global prices","الأسعار العالمية")} — {plan.name}</h2><button onClick={onClose}>×</button></div><p className="mt-3 text-sm text-slate-600">{text("The existing plan price remains the default. Country prices override general currency prices. These are explicit prices, with tax added at the configured rate.","يبقى سعر الخطة الحالي هو السعر الأساسي. تتقدم أسعار البلدان على أسعار العملات العامة، وتضاف الضريبة بالسعر المحدد.")}</p>
 {error&&<p role="alert" className="mt-3 text-red-700">{error}</p>}{notice&&<p role="status" className="mt-3 text-green-700">{notice}</p>}
 {config&&<><div className="mt-4 space-y-4">{config.prices.map((r,i)=><div key={i} className="grid gap-3 rounded-xl border p-4 sm:grid-cols-4">
 <label>{text("Currency","العملة")}<select className={field+" w-full"} value={r.currency} onChange={e=>update(i,{currency:e.target.value,decimal_precision:undefined})}>{currencies.map(c=><option key={c.code}>{c.code}</option>)}</select></label>
 <label>{text("Country (blank = all)","البلد (فارغ = الكل)")}<input className={field+" w-full"} maxLength={2} value={r.country} onChange={e=>update(i,{country:e.target.value.toUpperCase()})}/></label>
 <label>{text("Billing period","مدة الفوترة")}<select className={field+" w-full"} value={r.billing_period} onChange={e=>update(i,{billing_period:e.target.value})}>{["monthly","yearly","lifetime"].map(p=><option key={p}>{p}</option>)}</select></label>
 <label>{text("Price before tax","السعر قبل الضريبة")}<input className={field+" w-full"} inputMode="decimal" value={r.amount} onChange={e=>update(i,{amount:e.target.value})}/></label>
 <label>{text("Tax (%)","الضريبة (%)")}<input type="number" min="0" max="100" step="0.01" className={field+" w-full"} value={r.tax_bps/100} onChange={e=>update(i,{tax_bps:Math.round(Number(e.target.value)*100)})}/></label>
 <label>{text("Decimal places","المنازل العشرية")}<input type="number" min="0" max={currencies.find(c=>c.code===r.currency)?.exponent??2} className={field+" w-full"} value={r.decimal_precision??""} placeholder="ISO" onChange={e=>update(i,{decimal_precision:e.target.value===""?undefined:Number(e.target.value)})}/></label>
 <label>{text("Rounding","التقريب")}<select className={field+" w-full"} value={r.rounding||"reject"} onChange={e=>update(i,{rounding:e.target.value})}>{["reject","half_up","down","up"].map(v=><option key={v}>{v}</option>)}</select></label>
 <button type="button" className="text-red-700" onClick={()=>setConfig({...config,prices:config.prices.filter((_,j)=>i!==j)})}>{text("Remove","إزالة")}</button>
 </div>)}</div>
 <button className={field+" mt-4"} onClick={()=>setConfig({...config,prices:[...config.prices,{currency:plan.currency,country:"",billing_period:"monthly",amount:"1",tax_bps:0}]})}>{text("Add price","إضافة سعر")}</button>
 <fieldset className="mt-4"><legend className="font-semibold">{text("Allowed providers","مزودو الدفع المسموحون")}</legend><p className="text-xs text-slate-500">{text("Selecting a provider does not activate an integration. Checkout also requires a configured adapter and merchant account.","اختيار مزود لا يفعل التكامل. يتطلب الدفع تكاملاً وحساب تاجر مهيأ.")}</p>{["razorpay","stripe","paypal"].map(p=><label key={p} className="me-4 inline-flex gap-2"><input type="checkbox" checked={config.providers.includes(p)} onChange={e=>setConfig({...config,providers:e.target.checked?[...config.providers,p]:config.providers.filter(v=>v!==p)})}/>{p}</label>)}</fieldset>
 <button disabled={saving} onClick={save} className="mt-5 rounded-xl bg-blue-600 px-5 py-2 font-semibold text-white">{text(saving?"Saving…":"Save prices",saving?"جار الحفظ…":"حفظ الأسعار")}</button></>}
 </section></div>;
}
