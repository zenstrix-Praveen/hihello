/** Only return to local customer routes, never a URL supplied by another site. */
export function safeCustomerDestination(value: string | null): string {
  if (!value || /[\\\u0000-\u0020]/.test(value)) return "/dashboard";
  const pathname = value.split(/[?#]/, 1)[0];
  // Reject encoded separators and dot segments before URL normalization.
  if (/%(?:2f|5c|2e|25)/i.test(pathname) || pathname.split("/").some(part => part === "." || part === "..")) return "/dashboard";
  if (pathname !== "/dashboard" && !pathname.startsWith("/dashboard/")) return "/dashboard";
  return value;
}

export function customerLoginDestination(search: string): string {
  const params = new URLSearchParams(search);
  if (params.has("next")) return safeCustomerDestination(params.get("next"));
  // Registration already uses plan/billing_period links for existing customers.
  const plan = params.get("plan");
  if (plan && /^[1-9]\d*$/.test(plan) && Number.isSafeInteger(Number(plan))) {
    const period = params.get("billing_period");
    const selected = new URLSearchParams({ plan });
    if (period && ["monthly", "yearly", "lifetime"].includes(period)) selected.set("billing_period", period);
    for(const key of ["currency","country","reviewed_amount"]){const value=params.get(key);if(value)selected.set(key,value);}
    return `/dashboard/billing?${selected}`;
  }
  return "/dashboard";
}

export function customerRegistrationUrl(destination: string): string {
  const url = new URL(safeCustomerDestination(destination), "http://local");
  const plan = url.searchParams.get("plan");
  if (url.pathname !== "/dashboard/billing" || !plan || !/^[1-9]\d*$/.test(plan)) return "/register";
  const params = new URLSearchParams({ plan });
  const period = url.searchParams.get("billing_period");
  if (period && ["monthly", "yearly"].includes(period)) params.set("billing_period", period);
  for(const key of ["currency","country","reviewed_amount"]){const value=url.searchParams.get(key);if(value)params.set(key,value);}
  return `/register?${params}`;
}

export function customerLoginUrl(destination: string): string {
  return `/login/customer?${new URLSearchParams({ next: safeCustomerDestination(destination) })}`;
}

export function planBillingDestination(planID: number, period: string): string {
  return `/dashboard/billing?${new URLSearchParams({ plan: String(planID), billing_period: period.toLowerCase() })}`;
}
