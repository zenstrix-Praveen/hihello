"use client";

import { useEffect, useId, useRef, useState } from 'react';
import { useAdminLocale } from '@/lib/admin-locale';
import { PlanCurrencyOption, searchPlanCurrencies } from '@/lib/plan-currencies';
import { getApiBaseUrl } from '@/lib/api-config';

type ProviderStatus={provider:string;mode?:string;supported_currencies?:string[];currency_configuration_source?:string;merchant_currencies_verified?:boolean};
export default function PlanCurrencySelect({value,existingCurrency,onChange}: {value:string;existingCurrency?:string;onChange:(code:string)=>void}) {
  const {t}=useAdminLocale();
  const id=useId();
  const input=useRef<HTMLInputElement>(null);
  const [open,setOpen]=useState(false);
  const [query,setQuery]=useState('');
  const [active,setActive]=useState(0);
  const [currencies,setCurrencies]=useState<PlanCurrencyOption[]>([]);
  const [provider,setProvider]=useState<ProviderStatus|null>(null);
  const [loadError,setLoadError]=useState(false);
  useEffect(()=>{let live=true;Promise.all([fetch(`${getApiBaseUrl()}/public/currencies`).then(r=>r.ok?r.json():Promise.reject()),fetch(`${getApiBaseUrl()}/public/payment-provider`).then(r=>r.ok?r.json():Promise.reject())]).then(([catalog,status])=>{if(live){setCurrencies(catalog.data||[]);setProvider(status);setLoadError(false);}}).catch(()=>{if(live)setLoadError(true);});return()=>{live=false;}},[]);
  const options=searchPlanCurrencies(query,t,currencies,existingCurrency);
  const selected=searchPlanCurrencies('',t,currencies,existingCurrency).find(item=>item.code===value);
  const selectedLabel=selected ? `${selected.code} — ${t(selected.name)}` : value;
  const index=Math.min(active,Math.max(0,options.length-1));
  function close(){setOpen(false);setQuery('');setActive(0);}
  function choose(code:string){onChange(code);close();}
  return <div className="relative text-sm" onBlur={event=>{if(!event.currentTarget.contains(event.relatedTarget as Node|null))close();}}>
    <label htmlFor={id}>{t('Currency')} *</label>
    <div className="relative mt-1">
      <input ref={input} id={id} role="combobox" aria-autocomplete="list" aria-expanded={open} aria-controls={`${id}-options`}
        aria-activedescendant={open&&options.length?`${id}-option-${index}`:undefined}
        aria-describedby={`${id}-help`} autoComplete="off" placeholder={t('Search currencies')}
        value={open?query:selectedLabel}
        onFocus={()=>{setOpen(true);setQuery('');setActive(0);}}
        onChange={event=>{setQuery(event.target.value);setActive(0);setOpen(true);}}
        onKeyDown={event=>{
          if(event.key==='ArrowDown'||event.key==='ArrowUp'){
            event.preventDefault();setOpen(true);
            setActive(current=>!open?0:options.length?(current+(event.key==='ArrowDown'?1:-1)+options.length)%options.length:0);
          } else if(event.key==='Enter'&&open){event.preventDefault();if(options[index])choose(options[index].code);}
          else if(event.key==='Escape'&&open){event.preventDefault();event.stopPropagation();close();}
        }}
        className="w-full rounded-xl border border-slate-300 bg-white py-2.5 ps-3 pe-9 text-start focus:outline-none focus:ring-2 focus:ring-blue-500"/>
      <button type="button" tabIndex={-1} aria-label={t('Select a currency')} onMouseDown={event=>event.preventDefault()}
        onClick={()=>{if(open)close();else{input.current?.focus();setOpen(true);}}}
        className="absolute inset-y-0 end-0 px-3 text-slate-500" aria-hidden="true">▾</button>
    </div>
    <input type="hidden" name="currency" value={value}/>
    {open&&<div className="absolute inset-x-0 top-[4.5rem] z-20 max-h-60 overflow-y-auto rounded-xl border border-slate-200 bg-white p-1 shadow-lg">
      <ul id={`${id}-options`} role="listbox" aria-label={t('Currency')}>
        {options.map((option,i)=><li key={option.code} id={`${id}-option-${i}`} role="option" aria-selected={option.code===value}
          onMouseDown={event=>event.preventDefault()} onClick={()=>choose(option.code)} onMouseEnter={()=>setActive(i)}
          className={`cursor-pointer rounded-lg px-3 py-2 ${i===index?'bg-blue-50 text-blue-800':'text-slate-700'}`}>
          <bdi dir="ltr" className="font-semibold">{option.code}</bdi><span> — {t(option.name)}</span>
        </li>)}
      </ul>
      {!options.length&&<p role="status" className="px-3 py-2 text-slate-500">{t('No currencies found')}</p>}
    </div>}
    <p id={`${id}-help`} className="mt-2 text-xs text-slate-500">{t('Changing currency does not convert the price. Enter the correct amount for the selected currency.')}</p>
    {loadError&&<p role="status" className="mt-2 text-xs text-amber-700">{t('Currency catalog or checkout status is unavailable. Save validation will be performed by the server.')}</p>}
    {provider&&<p role="status" className={`mt-2 text-xs ${provider.supported_currencies?.includes(value)?'text-emerald-700':'text-amber-700'}`}>
      {provider.supported_currencies?.includes(value)?t('Currency is in the application allowlist. Confirm it is enabled on the Razorpay merchant account.') :t('Checkout is not configured for this currency. Configure the backend currency allowlist after confirming merchant activation.')}
      {provider.currency_configuration_source&&<span className="block text-slate-500">{provider.currency_configuration_source}{provider.mode?` · ${provider.mode}`:''}</span>}
    </p>}
  </div>;
}
