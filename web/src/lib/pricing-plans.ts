export type PublicPlan = {
  pricing_unavailable?: boolean;
  billing_config?: import("./global-pricing").PriceConfig;
  subtotal?:string; tax?:string; quote_total?:string;
  id: number;
  code?: string;
  name: string;
  description: string;
  price: number;
  pricing_type?: "free" | "fixed" | "custom";
  currency: string;
  billing_period: string;
  billing_type?: string;
  trial_days: number;
  trial_mode?: "disabled" | "limited" | "unlimited";
  cta_text?: string;
  cta_url?: string;
  is_featured?: boolean;
  is_most_popular?: boolean;
  feature_limits?: { features?: string[]; limits?: Record<string, unknown> };
  _locale_fallback?: boolean;
};

const limitLabels: Record<string, string> = {
  digital_cards_limit: "Digital Cards",
  nfc_tags_limit: "NFC Tags",
  qr_codes_limit: "QR Codes",
  contacts_limit: "Contacts",
  storage_limit: "Storage",
  team_members_limit: "Team Members",
  card_templates_limit: "Card Templates",
};

const unavailableFeatures = [
  "lead capture", "contact export", "crm integration", "email signature", "wallet integration",
  "custom qr branding", "api access", "priority support", "bulk card creation", "team management",
  "card views and insights",
];

function words(value: string) {
  return value.replace(/[_-]+/g, " ").trim().replace(/\s+/g, " ").replace(/\b\w/g, letter => letter.toUpperCase())
    .replace(/\bQr\b/g, "QR").replace(/\bNfc\b/g, "NFC").replace(/\bApi\b/g, "API");
}

function isUnavailable(label: string) {
  const normalized = label.replace(/[_-]+/g, " ").replace(/_enabled$/i, "").toLowerCase();
  return unavailableFeatures.some(feature => normalized.includes(feature));
}

export function isFreePlan(plan: Pick<PublicPlan, "price">) {
  return Number(plan.price) === 0;
}

export function comparePlanPrices(left: PublicPlan, right: PublicPlan): -1 | 0 | 1 | null {
  if (isFreePlan(right) && Number(left.price) > 0) return 1;
  if (isFreePlan(left) && Number(right.price) > 0) return -1;
  if (left.currency.toUpperCase() !== right.currency.toUpperCase()) return null;
  const monthlyEquivalent = (plan: PublicPlan) => {
    const amount = Number(plan.price);
    if (!Number.isFinite(amount)) return null;
    if (billingPeriod(plan) === "yearly") return amount / 12;
    if (plan.billing_period.toLowerCase() === "lifetime" || plan.billing_period.toLowerCase() === "custom") return null;
    return amount;
  };
  const a = monthlyEquivalent(left), b = monthlyEquivalent(right);
  if (a === null || b === null) return null;
  return a > b ? 1 : a < b ? -1 : 0;
}

export function billingPeriod(plan: Pick<PublicPlan, "billing_period">): "monthly" | "yearly" | "other" {
  const value = plan.billing_period.toLowerCase();
  return value === "monthly" ? "monthly" : value === "yearly" || value === "annual" ? "yearly" : "other";
}

export function planChangeLabel(plan: PublicPlan, current: PublicPlan): string {
  const period = billingPeriod(plan), currentPeriod = billingPeriod(current);
  if (plan.name.trim().toLowerCase() === current.name.trim().toLowerCase() && plan.currency.toUpperCase() === current.currency.toUpperCase()
    && period !== "other" && currentPeriod !== "other" && period !== currentPeriod) return "Change Billing Period";
  const comparison = comparePlanPrices(plan, current);
  return comparison === 1 ? "Upgrade Plan" : comparison === -1 ? "Downgrade Plan" : "Change Plan";
}

export function plansForBillingPeriod(plans: PublicPlan[], period: "monthly" | "yearly") {
  return plans.filter(plan => isFreePlan(plan) || billingPeriod(plan) === "other" || billingPeriod(plan) === period);
}

export function prioritizeFreePlans(plans: PublicPlan[]) {
  return plans
    .map((plan, index) => ({ plan, index }))
    .sort((left, right) => Number(!isFreePlan(left.plan)) - Number(!isFreePlan(right.plan)) || left.index - right.index)
    .map(({ plan }) => plan);
}

export function formatPlanPrice(plan: PublicPlan, t: (value: string) => string, language: string) {
  if (plan.pricing_unavailable) return { amount: language === "ar" ? "السعر غير متاح بـ " + plan.currency : "Pricing unavailable in " + plan.currency, period: "" };
  if (plan.pricing_type === "custom" || plan.billing_period.toLowerCase() === "custom") return { amount: t("Custom"), period: "" };
  if (isFreePlan(plan)) return { amount: t("Free"), period: t("Forever") };
  try {
    const amount = new Intl.NumberFormat(language === "ar" ? "ar" : "en", { style: "currency", currency: plan.currency }).format(Number(plan.price));
    const period = billingPeriod(plan) === "monthly" ? t("per month")
      : billingPeriod(plan) === "yearly" ? t("per year")
        : plan.billing_period.toLowerCase() === "lifetime" ? t("one-time")
          : `${t("per")} ${t(words(plan.billing_period).toLowerCase())}`;
    return { amount, period };
  } catch {
    const fractionDigits = (() => { try { return new Intl.NumberFormat("en", {style:"currency",currency:plan.currency}).resolvedOptions().maximumFractionDigits; } catch { return 2; } })();
    return { amount: `${plan.currency} ${Number(plan.price).toFixed(fractionDigits)}`, period: `per ${words(plan.billing_period).toLowerCase()}` };
  }
}

export function planFeatureLabels(plan: PublicPlan, t: (value: string, params?: Record<string, string | number>) => string) {
  const limits = plan.feature_limits?.limits || {};
  const labels = new Map<string, string>();
  for (const raw of plan.feature_limits?.features || []) {
    if (typeof raw !== "string" || !raw.trim() || isUnavailable(raw)) continue;
    const label = /[_-]/.test(raw) ? words(raw) : raw.trim();
    labels.set(label.toLowerCase(), t(label));
  }

  const analytics = String(limits.analytics_level || "").toLowerCase();
  if (analytics === "basic") labels.set("basic analytics", t("Basic Analytics"));
  else if (!analytics && (plan.feature_limits?.features || []).some(value => value.toLowerCase() === "analytics")) labels.set("analytics", t("Analytics"));
  if (String(limits.branding_level || "").toLowerCase() === "custom") labels.set("custom branding", t("Custom Branding"));

  for (const [key, raw] of Object.entries(limits)) {
    if (key.endsWith("_unlimited")) {
      if (raw === true) {
        const limitKey = `${key.slice(0, -10)}_limit`;
        const base = limitLabels[limitKey];
        if (base) labels.set(`unlimited ${base}`.toLowerCase(), t("Unlimited {label}", { label: t(base) }));
      }
      continue;
    }
    if (key === "analytics_level" || key === "branding_level" || isUnavailable(key)) continue;
    const base = limitLabels[key];
    if (base) {
      const unlimited = limits[`${key.slice(0, -6)}_unlimited`] === true || raw === -1;
      labels.delete(base.toLowerCase());
      if (unlimited) labels.set(`unlimited ${base}`.toLowerCase(), t("Unlimited {label}", { label: t(base) }));
      else if (typeof raw === "number" && raw > 0) {
        const label = key === "storage_limit" ? t("{count} GB Storage", { count: raw }) : t("{count} {label}", { count: raw, label: t(base) });
        labels.set(label.toLowerCase(), label);
      }
      continue;
    }
    if (typeof raw === "boolean" && raw) {
      const label = key.replace(/_enabled$/, "");
      if (!isUnavailable(label)) labels.set(words(label).toLowerCase(), t(words(label)));
    }
  }
  return [...labels.values()];
}

export function planUsageLines(plan: PublicPlan, t: (value: string, params?: Record<string, string | number>) => string) {
  const limits = plan.feature_limits?.limits || {};
  const rows: string[] = [];
  for (const [key, label] of Object.entries(limitLabels)) {
    const value = limits[key];
    const unlimited = limits[`${key.slice(0, -6)}_unlimited`] === true || value === -1;
    if (unlimited) rows.push(t("Unlimited {label}", { label: t(label) }));
    else if (typeof value === "number") rows.push(value === 0 ? t("No {label}", { label: t(label) }) : key === "storage_limit" ? t("{count} GB Storage", { count: value }) : t("{count} {label}", { count: value, label: t(label) }));
  }
  return rows;
}

function planFamily(plan: PublicPlan) {
  return (plan.code || plan.name).trim().toLowerCase().replace(/[\s_.-]+(monthly|yearly|annual|year)$/i, "");
}

export function maximumAnnualSavings(plans: PublicPlan[]) {
  let maximum = 0;
  for (const annual of plans.filter(plan => billingPeriod(plan) === "yearly" && Number(plan.price) > 0)) {
    const monthly = plans.find(plan => billingPeriod(plan) === "monthly" && Number(plan.price) > 0 && planFamily(plan) === planFamily(annual) && plan.currency.toUpperCase() === annual.currency.toUpperCase() && (plan.billing_type || "per_user") === (annual.billing_type || "per_user"));
    if (!monthly) continue;
    const annualizedMonthly = Number(monthly.price) * 12;
    const savings = annualizedMonthly > 0 ? Math.round((annualizedMonthly - Number(annual.price)) / annualizedMonthly * 100) : 0;
    if (savings > maximum) maximum = savings;
  }
  return maximum;
}
