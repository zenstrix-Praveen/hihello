"use client";
import {useEffect,useMemo,useState} from "react";
import {getApiBaseUrl} from "@/lib/api-config";
// Current tender suggestions derived from the backend Unicode CLDR snapshot.
// The selector includes 249 ISO countries/territories; non-ISO CLDR regions are excluded.
// Antarctica has no currency suggestion. Refresh both snapshots together.
import countryCurrencies from "@/lib/country-currencies.json";

export type CurrencyChoice={currency:string;country:string};
type Catalog={data:{code:string;name:string}[];country:string;suggested_currency:string};
let catalogRequest:Promise<Catalog>|undefined;
let catalogExpires=0;
function loadCatalog(){
 if(!catalogRequest||Date.now()>catalogExpires){
  catalogExpires=Date.now()+30000;
  catalogRequest=fetch(getApiBaseUrl()+"/public/billing/currencies").then(async response=>{
   if(!response.ok)throw Error("Could not load currencies");
   const data=await response.json();
   if(!Array.isArray(data.data))throw Error("Invalid currency catalog");
   return data as Catalog;
  }).catch(error=>{catalogRequest=undefined;throw error;});
 }
 return catalogRequest;
}
export function useCurrencyChoice(initial?:CurrencyChoice){
 const [choice,setChoice]=useState<CurrencyChoice>(initial||{currency:"",country:""});
 useEffect(()=>{
  if(initial?.currency)return;
  try{
   const stored=JSON.parse(localStorage.getItem("dbizcard.billing.currency")||"null");
   if(stored&&typeof stored.currency==="string"&&typeof stored.country==="string")
    setChoice({currency:/^[A-Z]{3}$/.test(stored.currency)?stored.currency:"",country:stored.country in countryCurrencies?stored.country:""});
  }catch{}
 },[initial?.currency]);
 function update(next:CurrencyChoice){setChoice(next);try{localStorage.setItem("dbizcard.billing.currency",JSON.stringify(next));}catch{}}
 return {choice,update};
}
export default function CurrencySelection({choice,onChange,language}:{choice:CurrencyChoice;onChange:(choice:CurrencyChoice)=>void;language:string}){
 const [currencies,setCurrencies]=useState<Catalog["data"]>([]);
 const [location,setLocation]=useState<CurrencyChoice|null>(null);
 const [query,setQuery]=useState("");
 const [error,setError]=useState(false);
 const [retry,setRetry]=useState(0);
 useEffect(()=>{
  let active=true;setError(false);
  loadCatalog().then(data=>{if(active){setCurrencies(data.data);if(data.suggested_currency)setLocation({currency:data.suggested_currency,country:data.country});}})
   .catch(()=>{if(active)setError(true)});
  return()=>{active=false};
 },[retry]);
 const ar=language==="ar";
 const countries=useMemo(()=>{
  const names=new Intl.DisplayNames([ar?"ar":"en"],{type:"region"});
  return Object.keys(countryCurrencies).map(code=>({code,name:names.of(code)||code})).sort((a,b)=>a.name.localeCompare(b.name,ar?"ar":"en"));
 },[ar]);
 const filtered=countries.filter(item=>item.code===choice.country||(item.name+" "+item.code).toLocaleLowerCase().includes(query.toLocaleLowerCase().trim()));
 const suggestedCode=choice.country?(countryCurrencies as Record<string,string>)[choice.country]:location?.currency;
 const suggestion=suggestedCode&&currencies.some(item=>item.code===suggestedCode)?{currency:suggestedCode,country:choice.country||location?.country||""}:null;
 return <div className="mx-auto mt-6 flex max-w-3xl flex-wrap items-end justify-center gap-3">
  <label className="text-sm font-semibold">{ar?"العملة":"Currency"}<select aria-label={ar?"العملة":"Currency"} value={choice.currency} onChange={e=>onChange({...choice,currency:e.target.value})} className="mt-1 block max-w-72 rounded-xl border bg-white px-3 py-2">
   <option value="">{ar?"عملة الخطة الأساسية":"Plan base currency"}</option>
   {choice.currency&&!currencies.some(c=>c.code===choice.currency)&&<option value={choice.currency}>{choice.currency}</option>}
   {currencies.map(c=><option key={c.code} value={c.code}>{c.code} — {c.name}</option>)}
  </select></label>
  <div className="space-y-1">
   <label className="block text-sm font-semibold" htmlFor="billing-country-search">{ar?"البحث عن بلد":"Search countries"}</label>
   <input id="billing-country-search" type="search" value={query} onChange={e=>setQuery(e.target.value)} placeholder={ar?"اسم البلد أو رمزه":"Country name or code"} className="block w-60 rounded-xl border bg-white px-3 py-2 text-sm"/>
   <label className="block text-sm font-semibold">{ar?"البلد":"Country"}<select aria-label={ar?"البلد":"Country"} value={choice.country} onChange={e=>{onChange({...choice,country:e.target.value});setQuery("");}} className="mt-1 block w-60 rounded-xl border bg-white px-3 py-2">
    <option value="">{ar?"اختر البلد":"Select country"}</option>
    {filtered.map(item=><option key={item.code} value={item.code}>{item.name} ({item.code})</option>)}
   </select></label>
   {filtered.length===0&&<p role="status" className="text-xs">{ar?"لم يتم العثور على بلد":"No matching countries"}</p>}
  </div>
  {suggestion&&suggestion.currency!==choice.currency&&<button type="button" onClick={()=>onChange(suggestion)} className="rounded-xl border bg-white px-3 py-2 text-sm">{ar?"العملة المقترحة:":"Suggested currency:"} {suggestion.currency}</button>}
  {error&&<p role="alert" className="w-full text-center text-sm text-red-700">{ar?"تعذر تحميل العملات.":"Could not load currencies."} <button type="button" onClick={()=>{catalogExpires=0;setRetry(value=>value+1)}} className="underline">{ar?"إعادة المحاولة":"Retry"}</button></p>}
  <p className="w-full text-center text-xs text-slate-500">{ar?"تتوفر الأسعار وطرق الدفع حسب إعدادات التاجر. تغيير البلد لا يغير عملتك المختارة.":"Prices and payment methods depend on merchant configuration. Changing country preserves your selected currency."}</p>
 </div>;
}

/** Pricing needs a currency choice only; country configuration remains available to billing. */
export function usePricingCurrency(){
 const [currency,setCurrency]=useState("");
 useEffect(()=>{
  try{
   const stored=JSON.parse(localStorage.getItem("dbizcard.billing.currency")||"null");
   if(typeof stored?.currency==="string"&&/^[A-Z]{3}$/.test(stored.currency))setCurrency(stored.currency);
  }catch{}
 },[]);
 function update(next:string){
  setCurrency(next);
  try{
   const stored=JSON.parse(localStorage.getItem("dbizcard.billing.currency")||"{}");
   localStorage.setItem("dbizcard.billing.currency",JSON.stringify({...stored,currency:next}));
  }catch{try{localStorage.setItem("dbizcard.billing.currency",JSON.stringify({currency:next}));}catch{}}
 }
 return {currency,update};
}
export function CurrencyDropdown({currency,onChange,language}:{currency:string;onChange:(currency:string)=>void;language:string}){
 const [currencies,setCurrencies]=useState<Catalog["data"]>([]);
 const [error,setError]=useState(false);
 const [retry,setRetry]=useState(0);
 useEffect(()=>{
  let active=true;setError(false);
  loadCatalog().then(data=>{if(active)setCurrencies(data.data)}).catch(()=>{if(active)setError(true)});
  return()=>{active=false};
 },[retry]);
 const ar=language==="ar";
 return <div className="mx-auto mt-6 w-full max-w-sm">
  <label className="block text-sm font-semibold">{ar?"العملة":"Currency"}<select aria-label={ar?"العملة":"Currency"} value={currency} onChange={event=>onChange(event.target.value)} className="mt-2 block w-full rounded-xl border border-slate-200 bg-white px-3 py-3 text-sm shadow-sm focus:outline-none focus:ring-2 focus:ring-violet-500">
   <option value="">{ar?"عملة الخطة الأساسية":"Plan base currency"}</option>
   {currency&&!currencies.some(item=>item.code===currency)&&<option value={currency}>{currency}</option>}
   {currencies.map(item=><option key={item.code} value={item.code}>{item.code} — {item.name}</option>)}
  </select></label>
  {error&&<p role="alert" className="mt-2 text-sm text-red-700">{ar?"تعذر تحميل العملات.":"Could not load currencies."} <button type="button" onClick={()=>{catalogExpires=0;setRetry(value=>value+1)}} className="underline">{ar?"إعادة المحاولة":"Retry"}</button></p>}
 </div>;
}
