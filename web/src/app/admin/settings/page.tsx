"use client";
import { getApiBaseUrl } from "@/lib/api-config";
import {useCallback,useEffect,useState} from "react";
import GlobalBillingSettings from "@/components/admin/GlobalBillingSettings";
import {adminApi} from "@/lib/admin-api";
import {useAdminLocation} from "@/lib/admin-location";
import { languageOptions, useAdminLocale } from "@/lib/admin-locale";

type Option={code:string;name:string};
type PaymentCurrency={code:string;name:string;symbol:string;exponent:number};
type PaymentCurrencySettings={provider:string;mode:string;provider_configured:boolean;currencies:PaymentCurrency[];supported_currencies:string[];published_supported_currencies?:string[];merchant_currencies_verified:boolean;currency_configuration_source:string};
type Settings={platform_name:string;support_email:string;support_phone:string;default_currency:string;timezone:string;date_format:string;time_format:string;default_country_code:string;default_country_name:string;default_state_code:string;default_state_name:string;default_city:string;postal_code:string;default_language:string;locale:string;enabled_languages:string[];logo_url:string;favicon_url:string;company_name:string;frontend_url:string;public_base_url:string;support_url:string;privacy_policy_url:string;terms_url:string;maintenance_mode:boolean;registration_enabled:boolean;default_user_status:string;default_card_status:string;default_template_id:number|null;payment_provider?:string;razorpay_mode?:string;razorpay_enabled?:boolean;razorpay_key_id_configured?:boolean;razorpay_key_secret_configured?:boolean;razorpay_webhook_secret_configured?:boolean;razorpay_supported_currencies?:string[];razorpay_currency_configuration_source?:string;razorpay_merchant_currencies_verified?:boolean};
const field="mt-1 w-full rounded-xl border border-slate-300 px-3 py-2.5";
const tabs=["General","Location","Localization","Currency","Branding","URLs","Operational","Email","Payment"] as const;

export default function SettingsPage(){
 const {search}=useAdminLocation();
 const {language,setLanguage,t}=useAdminLocale();
 const [tab,setTab]=useState<typeof tabs[number]>("General"),[settings,setSettings]=useState<Settings|null>(null),[countries,setCountries]=useState<Option[]>([]),[states,setStates]=useState<Option[]>([]),[languages,setLanguages]=useState<Option[]>([]),[templates,setTemplates]=useState<{id:number;name:string}[]>([]),[loading,setLoading]=useState(true),[saving,setSaving]=useState(false),[error,setError]=useState(""),[notice,setNotice]=useState("");

 const [convertAmount, setConvertAmount] = useState("1");
 const [fromCurrency, setFromCurrency] = useState("USD");
 const [toCurrency, setToCurrency] = useState("INR");
 const [convertedAmount, setConvertedAmount] = useState<number | null>(null);
 const [exchangeRate, setExchangeRate] = useState<number | null>(null);
 const [converting, setConverting] = useState(false);
 const [conversionError, setConversionError] = useState("");
 const [rateUpdatedAt, setRateUpdatedAt] = useState("");
 const [paymentCurrencySettings,setPaymentCurrencySettings]=useState<PaymentCurrencySettings|null>(null),[paymentCurrencySelection,setPaymentCurrencySelection]=useState<string[]>([]),[merchantConfirmed,setMerchantConfirmed]=useState(false),[paymentSettingsLoading,setPaymentSettingsLoading]=useState(false),[paymentSettingsSaving,setPaymentSettingsSaving]=useState(false),[paymentSettingsError,setPaymentSettingsError]=useState(""),[paymentSettingsNotice,setPaymentSettingsNotice]=useState("");
 const load=useCallback(async()=>{setLoading(true);setError("");try{const [s,c,l,t]=await Promise.all([adminApi("/admin/settings"),adminApi("/admin/settings/countries"),adminApi("/admin/settings/languages"),adminApi("/admin/templates")]);setSettings(s.data);setCountries(c.data);setLanguages(l.data);setTemplates(Array.isArray(t)?t:t.data??[])}catch(e){setError(e instanceof Error?e.message:"Could not load platform settings")}finally{setLoading(false)}},[]);
 useEffect(()=>{void load()},[load]);
 useEffect(()=>{const requested=new URLSearchParams(search).get("tab")||"General";const selected=tabs.find(t=>t.toLowerCase()===requested.toLowerCase());setTab(selected??"General")},[search]);
 useEffect(()=>{if(!settings?.default_country_code)return;adminApi(`/admin/settings/states?country=${encodeURIComponent(settings.default_country_code)}`).then(x=>setStates(x.data??[])).catch(e=>setError(e instanceof Error?e.message:"Could not load state options"))},[settings?.default_country_code]);
 useEffect(()=>{if(tab!=="Payment")return;let active=true;setPaymentSettingsLoading(true);setPaymentSettingsError("");adminApi("/admin/settings/payment-currencies").then(response=>{if(active){const data=response.data as PaymentCurrencySettings;setPaymentCurrencySettings(data);setPaymentCurrencySelection(data.supported_currencies??[]);setMerchantConfirmed(data.merchant_currencies_verified===true)}}).catch(e=>{if(active)setPaymentSettingsError(e instanceof Error?e.message:"Could not load payment currency settings")}).finally(()=>{if(active)setPaymentSettingsLoading(false)});return()=>{active=false}},[tab]);
 function selectTab(next:typeof tabs[number]){setTab(next);const url=new URL(window.location.href);url.searchParams.set("tab",next);window.history.pushState({},"",url);window.dispatchEvent(new PopStateEvent("popstate"))}
 function set<K extends keyof Settings>(key:K,value:Settings[K]){setSettings(s=>s?{...s,[key]:value}:s)}
 async function save(e:React.FormEvent){e.preventDefault();if(!settings)return;setSaving(true);setError("");setNotice("");try{const v=await adminApi("/admin/settings",{method:"PUT",body:JSON.stringify({...settings,default_template_id:settings.default_template_id?Number(settings.default_template_id):null})});setSettings(v.data);setNotice("Platform settings saved.")}catch(e){setError(e instanceof Error?e.message:"Could not save platform settings")}finally{setSaving(false)}}
 async function savePaymentCurrencies(){setPaymentSettingsSaving(true);setPaymentSettingsError("");setPaymentSettingsNotice("");try{const response=await adminApi("/admin/settings/payment-currencies",{method:"PUT",body:JSON.stringify({currencies:paymentCurrencySelection,merchant_confirmed:merchantConfirmed})});setPaymentCurrencySettings(response.data);setPaymentCurrencySelection(response.data.supported_currencies??[]);setMerchantConfirmed(response.data.merchant_currencies_verified===true);setPaymentSettingsNotice("Payment currency settings saved.")}catch(e){setPaymentSettingsError(e instanceof Error?e.message:"Could not save payment currency settings")}finally{setPaymentSettingsSaving(false)}}
 async function convertCurrency(){
   const amount = Number(convertAmount);

   if (!Number.isFinite(amount) || amount <= 0) {
     setConversionError("Enter a valid amount.");
     return;
   }

   if (fromCurrency === toCurrency) {
     setExchangeRate(1);
     setConvertedAmount(amount);
     setRateUpdatedAt(new Date().toISOString());
     setConversionError("");
     return;
   }

   setConverting(true);
   setConversionError("");

   try {
     const apiUrl = getApiBaseUrl();
     const params = new URLSearchParams({amount:String(amount),from:fromCurrency,to:toCurrency});
     const response = await fetch(`${apiUrl}/public/currency/convert?${params}`);

     if (!response.ok) {
       const errorData = await response.json().catch(() => null);
       if (response.status === 400) throw new Error(errorData?.error || "Check the amount and currencies, then try again.");
       if (response.status === 422) throw new Error("That currency pair is not supported. Choose another currency.");
       throw new Error(errorData?.error || "Exchange rates are temporarily unavailable. Please try again shortly.");
     }

     const data = await response.json();
     const converted = Number(data?.converted_amount);

     if (!Number.isFinite(converted)) {
       throw new Error("Invalid exchange-rate response");
     }

     setConvertedAmount(converted);
     setExchangeRate(Number(data.rate));
     setRateUpdatedAt(data.updated_at || "");
   } catch (error) {
     setConvertedAmount(null);
     setExchangeRate(null);
     setConversionError(
       error instanceof Error
         ? error.message
         : "Currency conversion failed. Please try again."
     );
   } finally {
     setConverting(false);
   }
 }
 function countryChange(code:string){if(!settings)return;const country=countries.find(c=>c.code===code);setSettings({...settings,default_country_code:code,default_country_name:country?.name??settings.default_country_name,default_state_code:"",default_state_name:""})}
 function toggleLanguage(code:string){if(!settings)return;const enabled=settings.enabled_languages.includes(code);const next=enabled?settings.enabled_languages.filter(x=>x!==code):[...settings.enabled_languages,code];set("enabled_languages",next);if(!next.includes(settings.default_language)&&next.length)set("default_language",next[0])}
 if(loading)return <section className="max-w-6xl"><p className="text-sm text-slate-500">Platform configuration</p><h1 className="mt-1 text-3xl font-bold">Settings</h1><div className="mt-6 rounded-2xl border bg-white p-12 text-center text-slate-500">Loading platform settings…</div></section>;
 if(!settings)return <section className="max-w-6xl"><h1 className="text-3xl font-bold">Settings</h1><div role="alert" className="mt-6 rounded-xl bg-red-50 p-4 text-red-800">{error||"Settings unavailable"}<button onClick={()=>void load()} className="ml-3 underline">Retry</button></div></section>;
 const label=(name:string,control:React.ReactNode)=><label className="block text-sm font-medium">{name}{control}</label>;
 const input=(key:keyof Settings,opts:React.InputHTMLAttributes<HTMLInputElement>={})=><input {...opts} value={String(settings[key]??"")} onChange={e=>set(key,e.target.value as never)} className={field}/>;
 return <section className="max-w-6xl"><p className="text-sm text-slate-500">Platform configuration</p><h1 className="mt-1 text-3xl font-bold">Settings</h1><p className="mt-2 text-sm text-slate-500">Manage platform defaults, localization, branding and operational behavior.</p>
 {notice&&<div role="status" className="mt-5 rounded-xl border border-green-200 bg-green-50 p-3 text-green-800">{notice}</div>}{error&&<div role="alert" className="mt-5 rounded-xl border border-red-200 bg-red-50 p-3 text-red-800">{error}</div>}
 <form onSubmit={save} className="mt-6 overflow-hidden rounded-2xl border bg-white"><div className="flex flex-wrap border-b bg-slate-50">{tabs.map(t=><button type="button" key={t} onClick={()=>selectTab(t)} className={`border-b-2 px-4 py-3 text-sm font-semibold ${tab===t?"border-blue-600 text-blue-700":"border-transparent text-slate-500"}`}>{t}</button>)}</div><div className="p-5 sm:p-7">
 {tab==="General"&&<div className="grid gap-4 sm:grid-cols-2">{label("Platform Name *",input("platform_name",{required:true,maxLength:150}))}{label("Company Name",input("company_name",{maxLength:150}))}{label("Support Email",input("support_email",{type:"email"}))}{label("Support Phone",input("support_phone",{type:"tel"}))}{label("Default Currency *",<select value={settings.default_currency} onChange={e=>set("default_currency",e.target.value)} className={field}>{["USD","EUR","GBP","INR","CAD","AUD","SGD","JPY","CNY","BRL","ZAR"].map(c=><option key={c}>{c}</option>)}</select>)}{label("Timezone *",input("timezone",{required:true,placeholder:"UTC or America/New_York"}))}{label("Date Format",<select value={settings.date_format} onChange={e=>set("date_format",e.target.value)} className={field}>{["YYYY-MM-DD","DD/MM/YYYY","MM/DD/YYYY","DD-MM-YYYY"].map(x=><option key={x}>{x}</option>)}</select>)}{label("Time Format",<select value={settings.time_format} onChange={e=>set("time_format",e.target.value)} className={field}><option value="24h">24 hour</option><option value="12h">12 hour</option></select>)}</div>}
 {tab==="Location"&&<div className="grid gap-4 sm:grid-cols-2">{label("Country *",<select required value={settings.default_country_code} onChange={e=>countryChange(e.target.value)} className={field}><option value="">Choose country</option>{countries.map(c=><option key={c.code} value={c.code}>{c.name}</option>)}</select>)}{settings.default_country_code==="ZZ"&&<>{label("Country Code *",input("default_country_code",{required:true,maxLength:2,pattern:"[A-Za-z]{2}",onChange:e=>set("default_country_code",e.target.value.toUpperCase())}))}{label("Country Name *",input("default_country_name",{required:true}))}</>}{label("State / Province",states.length>0?<select value={settings.default_state_code} onChange={e=>{const s=states.find(x=>x.code===e.target.value);setSettings({...settings,default_state_code:e.target.value,default_state_name:s?.name??""})}} className={field}><option value="">Select a state or enter below</option>{states.map(s=><option key={s.code} value={s.code}>{s.name}</option>)}<option value="CUSTOM">Other / enter manually</option></select>:input("default_state_name",{placeholder:"State or province"}))}{(states.length===0||settings.default_state_code==="CUSTOM")&&label("State / Province Name",input("default_state_name",{placeholder:"State or province"}))}{label("State Code",input("default_state_code",{placeholder:"Optional region code"}))}{label("City",input("default_city",{placeholder:"Enter city"}))}{label("Postal Code",input("postal_code",{autoComplete:"postal-code"}))}<p className="sm:col-span-2 text-xs text-slate-500">Country and state options are extensible; locations outside the listed state options can be entered directly.</p></div>}
 {tab==="Localization"&&<div className="grid gap-4 sm:grid-cols-2">{label("Admin UI Language",<select aria-label="Admin UI Language" value={language} onChange={e=>setLanguage(e.target.value)} className={field}>{languageOptions.map(l=><option key={l.code} value={l.code}>{l.name}</option>)}</select>)}{label("Default Language *",<select required value={settings.default_language} onChange={e=>set("default_language",e.target.value)} className={field}>{languages.filter(l=>settings.enabled_languages.includes(l.code)).map(l=><option key={l.code} value={l.code}>{l.name}</option>)}</select>)}{label("Locale *",input("locale",{required:true,placeholder:"en-US"}))}{label("Timezone *",input("timezone",{required:true,placeholder:"UTC"}))}{label("Currency *",input("default_currency",{required:true,maxLength:3,pattern:"[A-Za-z]{3}"}))}<div className="sm:col-span-2"><p className="text-sm font-medium">Enabled Languages *</p><div className="mt-2 flex flex-wrap gap-3">{languageOptions.map(l=><label key={l.code} className="flex items-center gap-2 rounded-xl border px-3 py-2 text-sm"><input type="checkbox" checked={settings.enabled_languages.includes(l.code)} onChange={()=>toggleLanguage(l.code)}/>{l.name} ({l.code})</label>)}</div></div></div>}
 {tab==="Currency"&&<div className="space-y-6">
<div className="grid gap-4 sm:grid-cols-2">
{label(t("Default Currency *"),<select required value={settings.default_currency} onChange={e=>set("default_currency",e.target.value)} className={field}>{["USD","EUR","GBP","INR","CAD","AUD","SGD","JPY","CNY","BRL","ZAR"].map(c=><option key={c} value={c}>{c}</option>)}</select>)}
{label(t("Timezone *"),input("timezone",{required:true,placeholder:"UTC or America/New_York"}))}
{label(t("Date Format"),<select value={settings.date_format} onChange={e=>set("date_format",e.target.value)} className={field}>{["YYYY-MM-DD","DD/MM/YYYY","MM/DD/YYYY","DD-MM-YYYY"].map(x=><option key={x}>{x}</option>)}</select>)}
{label(t("Time Format"),<select value={settings.time_format} onChange={e=>set("time_format",e.target.value)} className={field}><option value="24h">{t("24 hour")}</option><option value="12h">{t("12 hour")}</option></select>)}
</div>

<div className="rounded-2xl border bg-slate-50 p-5">
<h2 className="text-lg font-bold">{t("Currency Converter")}</h2>
<p className="mt-1 text-sm text-slate-500">{t("Convert an amount between supported currencies using the latest available exchange rate.")}</p>

<div className="mt-5 grid gap-4 md:grid-cols-3">
<label className="block text-sm font-medium">
{t("Amount")}
<input type="number" min="0" step="0.01" value={convertAmount} onChange={e=>setConvertAmount(e.target.value)} className={field}/>
</label>

<label className="block text-sm font-medium">
{t("From Currency")}
<select value={fromCurrency} onChange={e=>setFromCurrency(e.target.value)} className={field}>
{["USD","EUR","GBP","INR","CAD","AUD","SGD","JPY","CNY","BRL","ZAR"].map(currency=><option key={currency} value={currency}>{currency}</option>)}
</select>
</label>

<label className="block text-sm font-medium">
{t("To Currency")}
<select value={toCurrency} onChange={e=>setToCurrency(e.target.value)} className={field}>
{["USD","EUR","GBP","INR","CAD","AUD","SGD","JPY","CNY","BRL","ZAR"].map(currency=><option key={currency} value={currency}>{currency}</option>)}
</select>
</label>
</div>

<button type="button" onClick={convertCurrency} disabled={converting} className="mt-5 rounded-xl bg-blue-600 px-5 py-2.5 font-semibold text-white disabled:opacity-50">
{converting?t("Converting..."):t("Convert Currency")}
</button>

{conversionError&&<p className="mt-4 text-sm text-red-600">{conversionError}</p>}

{convertedAmount!==null&&<div className="mt-5 grid gap-4 sm:grid-cols-2">
<div className="rounded-xl border bg-white p-4">
<p className="text-xs font-medium text-slate-500">{t("Exchange Rate")}</p>
<p className="mt-1 text-lg font-bold">1 {fromCurrency} = {exchangeRate?.toFixed(6)} {toCurrency}</p>{rateUpdatedAt&&<p className="mt-1 text-xs text-slate-500">Updated {new Date(rateUpdatedAt).toLocaleString()}</p>}
</div>

<div className="rounded-xl border bg-white p-4">
<p className="text-xs font-medium text-slate-500">{t("Converted Amount")}</p>
<p className="mt-1 text-lg font-bold">{new Intl.NumberFormat(undefined,{style:"currency",currency:toCurrency}).format(convertedAmount)}</p>
</div>
</div>}
</div>
</div>}
 {tab==="Branding"&&<div className="grid gap-4 sm:grid-cols-2">{label("Company Name",input("company_name"))}{label("Logo URL",input("logo_url",{type:"url",placeholder:"https://..."}))}{label("Favicon URL",input("favicon_url",{type:"url",placeholder:"https://..."}))}</div>}
 {tab==="URLs"&&<div className="grid gap-4 sm:grid-cols-2">{label("Frontend URL",input("frontend_url",{type:"url",placeholder:"https://..."}))}{label("Public Base URL",input("public_base_url",{type:"url",placeholder:"https://..."}))}{label("Support URL",input("support_url",{type:"url",placeholder:"https://..."}))}{label("Privacy Policy URL",input("privacy_policy_url",{type:"url",placeholder:"https://..."}))}{label("Terms URL",input("terms_url",{type:"url",placeholder:"https://..."}))}</div>}
 {tab==="Operational"&&<div className="grid gap-4 sm:grid-cols-2"><label className="flex items-center gap-3 rounded-xl border p-4 text-sm"><input type="checkbox" checked={settings.maintenance_mode} onChange={e=>set("maintenance_mode",e.target.checked)}/>Maintenance Mode</label><label className="flex items-center gap-3 rounded-xl border p-4 text-sm"><input type="checkbox" checked={settings.registration_enabled} onChange={e=>set("registration_enabled",e.target.checked)}/>Registration Enabled</label>{label("Default User Status",<select value={settings.default_user_status} onChange={e=>set("default_user_status",e.target.value)} className={field}><option value="active">Active</option><option value="pending">Pending</option></select>)}{label("Default Card Status",<select value={settings.default_card_status} onChange={e=>set("default_card_status",e.target.value)} className={field}><option value="draft">Draft</option><option value="published">Published</option></select>)}{label("Default Template",<select value={settings.default_template_id??""} onChange={e=>set("default_template_id",e.target.value?Number(e.target.value):null)} className={field}><option value="">No default template</option>{templates.map(t=><option key={t.id} value={t.id}>{t.name}</option>)}</select>)}</div>}
 {tab==="Email"&&<div className="rounded-xl border border-amber-200 bg-amber-50 p-5 text-sm text-amber-900"><h2 className="font-semibold">Email delivery configuration</h2><p className="mt-2">Email credentials are not stored in platform settings or exposed to this browser. Configure the mail provider through backend environment variables and deployment secrets. No email delivery provider is currently wired into the admin API.</p></div>}
 {tab==="Payment"&&<div className="rounded-xl border bg-white p-5"><h2 className="font-semibold">{t("Payment provider")}</h2><div className="mt-4 grid gap-3 sm:grid-cols-2">{[["Provider",settings.payment_provider||"Razorpay"],["Status",settings.razorpay_enabled?t("Enabled"):t("Not configured")],["Mode",settings.razorpay_mode||"test"],["Key ID",settings.razorpay_key_id_configured?t("Configured"):t("Not configured")],["Key Secret",settings.razorpay_key_secret_configured?t("Configured"):t("Not configured")],["Webhook Secret",settings.razorpay_webhook_secret_configured?t("Configured"):t("Not configured")]].map(([label,value])=><div key={label} className="rounded-xl border bg-slate-50 p-3"><p className="text-xs font-medium text-slate-500">{t(label)}</p><p className="mt-1 text-sm font-semibold">{value}</p></div>)}</div><p className="mt-4 text-sm"><strong>{t("Configured checkout currency allowlist")}:</strong> {(settings.razorpay_supported_currencies||[]).join(", ")||t("None")}</p><p className="mt-2 text-sm text-amber-800">{t("This is an application allowlist only. Confirm each currency is enabled for the Razorpay merchant account before adding it to RAZORPAY_SUPPORTED_CURRENCIES. Merchant activation is not automatically verified by the application.")}</p><p className="mt-2 text-xs text-slate-500">{settings.razorpay_currency_configuration_source}</p><p className="mt-3 text-sm text-slate-500">{t("Razorpay credentials are read from backend environment variables. Secret values are never returned to this page.")}</p></div>}
 {tab==="Payment"&&<section className="mt-4 rounded-xl border bg-white p-5"><h2 className="font-semibold">{t("Merchant-enabled checkout currencies")}</h2><p className="mt-2 text-sm text-slate-600">{t("Choose only currencies already enabled for this Razorpay merchant account. The backend will use this saved list for upgrade availability and checkout.")}</p>{paymentSettingsLoading&&<p role="status" className="mt-4 text-sm text-slate-500">{t("Loading payment currencies…")}</p>}{paymentSettingsError&&<p role="alert" className="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700">{paymentSettingsError}</p>}{paymentCurrencySettings&&<><div className="mt-4 flex flex-wrap gap-3 text-sm"><span className="rounded-lg border bg-slate-50 px-3 py-2">{t("Provider")}: <strong>{paymentCurrencySettings.provider}</strong></span><span className="rounded-lg border bg-slate-50 px-3 py-2">{t("Mode")}: <strong>{paymentCurrencySettings.mode}</strong></span><span className={`rounded-lg border px-3 py-2 ${paymentCurrencySettings.provider_configured?"bg-emerald-50 text-emerald-800":"bg-amber-50 text-amber-800"}`}>{paymentCurrencySettings.provider_configured?t("Configured"):t("Not configured")}</span></div><p className="mt-4 text-sm font-semibold">{t("Merchant-enabled currencies")}</p><div className="mt-2 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">{paymentCurrencySettings.currencies.map(currency=><label key={currency.code} className="flex min-h-11 items-center gap-3 rounded-lg border p-3 text-sm"><input type="checkbox" disabled={!!paymentCurrencySettings.published_supported_currencies&&!paymentCurrencySettings.published_supported_currencies.includes(currency.code)} checked={paymentCurrencySelection.includes(currency.code)} onChange={event=>setPaymentCurrencySelection(value=>event.target.checked?[...value,currency.code]:value.filter(code=>code!==currency.code))}/><span><bdi dir="ltr" className="font-semibold">{currency.code} {currency.symbol}</bdi><span className="ms-1 text-slate-600">{t(currency.name)} · {currency.exponent} {t("decimal places")}</span></span></label>)}</div><label className="mt-4 flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-950"><input className="mt-1" type="checkbox" checked={merchantConfirmed} onChange={event=>setMerchantConfirmed(event.target.checked)}/><span>{t("I verified these currencies are enabled for the Razorpay merchant account. Selecting a currency here records that confirmation; it does not enable unsupported merchant currencies.")}</span></label><p className="mt-3 text-xs text-slate-500">{paymentCurrencySettings.currency_configuration_source}</p>{paymentSettingsNotice&&<p role="status" className="mt-3 text-sm text-emerald-700">{t(paymentSettingsNotice)}</p>}<button type="button" disabled={paymentSettingsSaving||paymentSettingsLoading||(paymentCurrencySelection.length>0&&!merchantConfirmed)} onClick={()=>void savePaymentCurrencies()} className="mt-4 min-h-11 rounded-xl bg-blue-600 px-5 py-2 font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50">{paymentSettingsSaving?t("Saving…"):t("Save currency settings")}</button></>}</section>}</div><div className="flex justify-end border-t bg-slate-50 px-5 py-4">{tab==="Email"||tab==="Payment"?<p className="text-sm text-slate-600">Payment currencies are saved separately after merchant verification.</p>:<button disabled={saving} className="rounded-xl bg-blue-600 px-5 py-2.5 font-semibold text-white disabled:opacity-50">{saving?"Saving…":"Save Settings"}</button>}</div></form>{tab==="Payment"&&<GlobalBillingSettings language={language}/>}
 </section>
}



