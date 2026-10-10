"use client";

import { useCallback, useEffect, useState } from 'react';
import { usePathname } from 'next/navigation';
import { LocaleProvider, useLocale } from './locale-provider';
import { adminTranslations } from './locale-catalogs';
import { customerCommonMessages } from './customer-common-messages';
import { customerAuthMessages } from './customer-auth-messages';
import { customerCardMessages } from './customer-card-messages';
import { customerWorkspaceMessages } from './customer-workspace-messages';
import { customerChatMessages } from './customer-chat-messages';
import { customerPublicMessages } from './customer-public-messages';
import { customerStoryMessages } from './customer-story-messages';

const languages = [{code:'en',name:'English'},{code:'ar',name:'العربية'}];
export const customerTranslations = {en:adminTranslations.en, ar:{"The plan total changed. Review the updated amount and confirm again.":"تغير المبلغ الإجمالي للخطة. راجع المبلغ المحدث وأكد مرة أخرى.","The payment provider returned a different currency. Refresh billing or contact support.":"أعاد مزود الدفع عملة مختلفة. حدث صفحة الفوترة أو اتصل بالدعم.",...adminTranslations.ar,...customerCommonMessages,...customerAuthMessages,...customerCardMessages,...customerWorkspaceMessages,...customerChatMessages,...customerPublicMessages,...customerStoryMessages}};
export function CustomerLocaleBoundary({children}: {children:React.ReactNode}) {
  const pathname = usePathname();
  // Public, authentication and customer routes share one persistent frontend locale.
  // Admin localization remains isolated in its existing AdminLocaleProvider.
  if (pathname.startsWith('/admin') || pathname === '/admin-login') return children;
  return <LocaleProvider storageKey="frontend_language" languages={languages} catalogs={customerTranslations}><CustomerSurface>{children}</CustomerSurface></LocaleProvider>;
}

function CustomerSurface({children}: {children:React.ReactNode}) {
  const {language,t} = useLocale();
  const [fallbackSections,setFallbackSections] = useState<string[]>([]);
  useEffect(()=>{
    const update=(event:Event)=>{const detail=(event as CustomEvent<{section:string;fallback:boolean}>).detail;setFallbackSections(current=>detail.fallback?[...new Set([...current,detail.section])]:current.filter(item=>item!==detail.section));};
    window.addEventListener('website-locale-fallback',update);return()=>window.removeEventListener('website-locale-fallback',update);
  },[]);
  useEffect(() => {
    document.querySelectorAll<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>('[data-customer-locale] input, [data-customer-locale] select, [data-customer-locale] textarea').forEach(field=>field.setCustomValidity(''));
  }, [language]);
  return <div data-customer-locale={language} dir={language === 'ar' ? 'rtl' : 'ltr'} lang={language}
    onInvalidCapture={event=>{
      const field = event.target;
      if (!(field instanceof HTMLInputElement || field instanceof HTMLSelectElement || field instanceof HTMLTextAreaElement)) return;
      field.setCustomValidity('');
      const validity = field.validity;
      if (validity.valueMissing) field.setCustomValidity(t('This field is required'));
      else if (validity.typeMismatch) field.setCustomValidity(t(field instanceof HTMLInputElement && field.type==='email' ? 'Please enter a valid email address.' : 'Please enter a valid URL.'));
      else if (validity.tooShort && 'minLength' in field) field.setCustomValidity(t('Please enter at least {count} characters.',{count:field.minLength}));
      else if (!validity.valid) field.setCustomValidity(t('Please enter a valid value'));
    }}
    onInputCapture={event=>{const field=event.target;if(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement || field instanceof HTMLSelectElement) field.setCustomValidity('');}}
  >{language==='ar'&&fallbackSections.length>0&&<p role="status" className="border-b border-amber-200 bg-amber-50 px-4 py-2 text-center text-sm text-amber-950">{t('Some website content has no Arabic version yet. English content is shown for those sections.')}</p>}{children}</div>;
}

export function useCustomerLocale() {
  const locale = useLocale();
  const error = useCallback((value: unknown): string => {
    const message = value instanceof Error ? value.message : typeof value === 'string' ? value : '';
    if (!message) return locale.t('Something went wrong. Please try again.');
    if (locale.language !== 'ar') return message;
    const key = Object.keys(customerTranslations.ar).find(key=>key.toLowerCase()===message.trim().toLowerCase());
    if (key) return locale.t(key);
    const status = /^Request failed \((\d+)\)$/.exec(message);
    if (status) return locale.t('Request failed ({status})',{status:status[1]});
    // Backend messages are display-only. Unknown messages must not leak English or internal details.
    return locale.t('Something went wrong. Please try again.');
  }, [locale.language,locale.t]);
  return {...locale,error};
}
