export type LocaleParams = Record<string, string | number>;
export type LocaleCatalogs = Record<string, Record<string, string>>;
export function readLanguage(storage: Pick<Storage, 'getItem'> | undefined, key: string, allowed: readonly string[], fallback = 'en') {
  try { const value = storage?.getItem(key); return value && allowed.includes(value) ? value : fallback; } catch { return fallback; }
}
export function persistLanguage(storage: Pick<Storage, 'setItem'> | undefined, key: string, value: string) {
  try { storage?.setItem(key, value); } catch { /* Private browsing can disable storage. */ }
}
export function translate(catalogs: LocaleCatalogs, language: string, key: string, params?: LocaleParams): string {
  const message = catalogs[language]?.[key] ?? catalogs.en?.[key] ?? key;
  return params ? message.replace(/\{(\w+)\}/g, (match, name: string) => params[name] === undefined ? match : String(params[name])) : message;
}
