"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useCustomerLocale } from "@/lib/customer-locale";
import { getApiBaseUrl } from "@/lib/api-config";

/** Loads the public, safe projection of a CMS section without retaining stale Next/browser fetch data. */
export function useWebsiteSection<T extends Record<string, unknown>>(section: string, defaults: T, options: { fallbackOnUnavailable?: boolean } = {}): T {
  const { language } = useCustomerLocale();
  const defaultsRef = useRef(defaults);
  defaultsRef.current = defaults;
  const [content, setContent] = useState<T>(options.fallbackOnUnavailable ? defaults : { ...defaults, enabled: false });
  const unavailableFallback = useCallback(() => {
    if (language !== "ar") return defaultsRef.current;
    return { ...defaultsRef.current, _locale_fallback: true } as T;
  }, [language]);
  useEffect(() => {
    const controller = new AbortController();
    fetch(`${getApiBaseUrl()}/public/website/${section}?lang=${language}`, { cache: "no-store", signal: controller.signal })
      .then(async response => {
        if (response.status === 404) return options.fallbackOnUnavailable ? unavailableFallback() : { enabled: false };
        if (!response.ok) throw new Error("Could not load website content");
        return response.json();
      })
      .then(value => { if (!controller.signal.aborted && value && typeof value === "object") {
        window.dispatchEvent(new CustomEvent("website-locale-fallback", {detail:{section, fallback:language === "ar" && value._locale_fallback === true}}));
        if (value.enabled === false) setContent(value);
        else { const {_locale_fallback: fallback, ...localized} = value; setContent({...defaultsRef.current, ...localized, _locale_fallback:fallback} as T); }
      } })
      .catch(() => { if (!controller.signal.aborted && options.fallbackOnUnavailable) {
        const fallback = unavailableFallback();
        if (language === "ar") window.dispatchEvent(new CustomEvent("website-locale-fallback", { detail: { section, fallback: true } }));
        setContent(fallback);
      } });
    return () => controller.abort();
  }, [section, language, options.fallbackOnUnavailable, unavailableFallback]);
  return content;
}
