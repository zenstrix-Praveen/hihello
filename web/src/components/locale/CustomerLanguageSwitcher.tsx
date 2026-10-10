"use client";
import { useCustomerLocale } from '@/lib/customer-locale';

export default function CustomerLanguageSwitcher({className = ''}: {className?:string}) {
  const {language,setLanguage,t} = useCustomerLocale();
  return <label className={`inline-flex shrink-0 items-center ${className}`}>
    <span className="sr-only">{t('Language')}</span>
    <select aria-label={t('Language')} value={language} onChange={event=>setLanguage(event.target.value)}
      className="min-h-10 rounded-xl border border-slate-200 bg-white px-2 py-2 text-sm font-semibold text-slate-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">
      <option value="en" lang="en">English</option><option value="ar" lang="ar">العربية</option>
    </select>
  </label>;
}
