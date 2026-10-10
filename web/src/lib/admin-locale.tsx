"use client";
import { adminApi } from "@/lib/admin-api";
import { LocaleProvider, useLocale } from "./locale-provider";
import { adminTranslations, languageOptions } from "./locale-catalogs";
export { languageOptions } from "./locale-catalogs";
const loadAdminDefault = () => adminApi("/admin/settings").then(response => response.data?.default_language || "en");
export function AdminLocaleProvider({ children }: { children: React.ReactNode }) {
  return <LocaleProvider storageKey="admin_language" languages={languageOptions} catalogs={adminTranslations} loadDefault={loadAdminDefault}>{children}</LocaleProvider>;
}
export function useAdminLocale() { return useLocale(); }