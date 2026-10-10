const publicRoutes = new Set([
  "/", "/features", "/platform", "/templates", "/pricing", "/business", "/contact",
  "/login", "/login/customer", "/register", "/admin/login", "/about", "/faq", "/signin",
]);

export function navigationPath(href: string): string {
  try {
    return new URL(href, "http://navigation.invalid").pathname.replace(/\/+$/, "") || "/";
  } catch {
    return "";
  }
}

/** CMS links to this frontend must keep the visitor's origin (including LAN port). */
export function publicNavigationHref(value: string, siteURL?: string, currentOrigin?: string): string {
  const href = value.trim();
  if (!/^(https?:)?\/\//i.test(href)) return href;
  try {
    const url = new URL(href, currentOrigin || "http://navigation.invalid");
    const loopback = ["localhost", "127.0.0.1", "[::1]"].includes(url.hostname);
    let siteOrigin: string | undefined;
    try { siteOrigin = siteURL ? new URL(siteURL).origin : undefined; } catch { /* An invalid SEO URL cannot change routing. */ }
    if (url.origin === currentOrigin || url.origin === siteOrigin || (loopback && publicRoutes.has(navigationPath(href)))) {
      return `${url.pathname}${url.search}${url.hash}`;
    }
  } catch { /* Preserve unsupported URLs; do not guess an internal route. */ }
  return href;
}
