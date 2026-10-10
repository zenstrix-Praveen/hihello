"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';
import { persistLanguage, readLanguage, translate, type LocaleCatalogs, type LocaleParams } from './locale-core';

export type LanguageOption = { code: string; name: string };
type LocaleValue = { language: string; languages: LanguageOption[]; setLanguage: (value: string) => void; t: (key: string, params?: LocaleParams) => string; ready: boolean };
const LocaleContext = createContext<LocaleValue>({language:'en',languages:[],setLanguage:()=>{},t:(key,params)=>translate({},'en',key,params),ready:false});

export function LocaleProvider({children, storageKey, languages, catalogs, loadDefault}: {
  children: React.ReactNode; storageKey: string; languages: LanguageOption[]; catalogs: LocaleCatalogs; loadDefault?: () => Promise<string>;
}) {
  const [language, setLanguageState] = useState('en');
  const [ready, setReady] = useState(false);
  const apply = useCallback((value: string) => {
    if (!languages.some(option => option.code === value)) return;
    setLanguageState(value);
    document.documentElement.lang = value;
    document.documentElement.dir = value === 'ar' ? 'rtl' : 'ltr';
    try { persistLanguage(localStorage, storageKey, value); } catch { /* Storage access itself can throw. */ }
    setReady(true);
  }, [languages, storageKey]);
  useEffect(() => {
    let active = true;
    let stored = '';
    try { stored = readLanguage(localStorage, storageKey, languages.map(item=>item.code), ''); } catch { /* Fall back to default. */ }
    if (stored) apply(stored);
    else if (loadDefault) loadDefault().then(code => { if (active) apply(languages.some(item=>item.code===code) ? code : 'en'); }).catch(()=>{if(active) apply('en');});
    else apply('en');
    const sync = (event: StorageEvent) => { if (event.key === storageKey) apply(languages.some(item=>item.code===event.newValue) ? event.newValue! : 'en'); };
    window.addEventListener('storage', sync);
    return () => { active = false; window.removeEventListener('storage', sync); document.documentElement.lang = 'en'; document.documentElement.dir = 'ltr'; };
  }, [apply, languages, loadDefault, storageKey]);
  const t = useCallback((key: string, params?: LocaleParams) => translate(catalogs, language, key, params), [catalogs, language]);
  const value = useMemo(()=>({language,languages,setLanguage:apply,t,ready}),[language,languages,apply,t,ready]);
  return <LocaleContext.Provider value={value}>{children}</LocaleContext.Provider>;
}
export function useLocale() { return useContext(LocaleContext); }
