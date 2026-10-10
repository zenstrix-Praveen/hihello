"use client";

import { getApiBaseUrl } from "@/lib/api-config";
import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {CurrencyDropdown,usePricingCurrency} from "@/components/pricing/CurrencySelection";
import {pricedPlans} from "@/lib/global-pricing";
import PublicHeader from "@/components/public/PublicHeader";
import PublicFooter from "@/components/public/PublicFooter";
import PlanPricingCard from "@/components/pricing/PlanPricingCard";
import { apiRequest, getUserToken } from "@/lib/user-api";
import { customerLoginUrl, planBillingDestination } from "@/lib/customer-upgrade";
import { useWebsiteSection } from "@/components/public/useWebsiteSection";
import { useCustomerLocale } from "@/lib/customer-locale";
import { billingPeriod, formatPlanPrice, isFreePlan, maximumAnnualSavings, planChangeLabel, planFeatureLabels, plansForBillingPeriod, prioritizeFreePlans, PublicPlan } from "@/lib/pricing-plans";

type Subscription = { plan_id: number; status: string;billing_period?:string;currency?:string } | null;
const API = getApiBaseUrl();

export default function PricingPage() {
  const { t, language, error: localizeError } = useCustomerLocale();
  const content = useWebsiteSection<any>("pricing", { enabled: true }, { fallbackOnUnavailable: true });
  const router = useRouter();
  const {currency,update}=usePricingCurrency();
  const [basePlans, setPlans] = useState<PublicPlan[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [currentPlanId, setCurrentPlanId] = useState<number | null>(null);
  const [reloadPlans, setReloadPlans] = useState(0);
  const [selectedPeriod, setSelectedPeriod] = useState<"monthly" | "yearly">("monthly");
  const [choosing, setChoosing] = useState<number | null>(null);
  const choosingRef = useRef(false);
  const [actionError, setActionError] = useState("");
  const [currentTerms,setCurrentTerms]=useState<{billing_period?:string;currency?:string}>({});
  const [subscriptionStatus, setSubscriptionStatus] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);setError("");
    fetch(`${API}/website/pricing?lang=${language}`, { cache: "no-store", signal: controller.signal })
      .then(async response => {
        const data = await response.json();
        if (!response.ok) throw new Error(data?.error || "Could not load plans");
        const rows = Array.isArray(data?.data) ? data.data as PublicPlan[] : [];
        setPlans(rows);
        setSelectedPeriod(current => rows.some(plan => billingPeriod(plan) === current) ? current : rows.some(plan => billingPeriod(plan) === "monthly") ? "monthly" : "yearly");
      })
      .catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(localizeError(reason));
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [localizeError, language, reloadPlans]);

  useEffect(() => {
    if (!getUserToken()) return;
    apiRequest("/billing/history")
      .then((data: { current_subscription?: Subscription }) => {
        const subscription = data.current_subscription;
        setSubscriptionStatus(subscription?.status || "");
        setCurrentTerms(subscription||{});
        if (subscription && ["active", "trial", "trialing", "past_due"].includes(subscription.status)) setCurrentPlanId(Number(subscription.plan_id));
      })
      .catch(() => { /* Public plan browsing stays available if subscription lookup fails. */ });
  }, []);

  useEffect(() => {
    const fallback = basePlans.some(plan => plan._locale_fallback === true);
    window.dispatchEvent(new CustomEvent("website-locale-fallback", { detail: { section: "pricing-plans", fallback: language === "ar" && fallback } }));
  }, [basePlans, language]);

  const plans=useMemo(()=>pricedPlans(basePlans,currency,""),[basePlans,currency]);
  const hasMonthly = plans.some(plan => billingPeriod(plan) === "monthly");
  const hasYearly = plans.some(plan => billingPeriod(plan) === "yearly");
  const yearlySavings = useMemo(() => maximumAnnualSavings(plans.filter(plan=>!plan.pricing_unavailable)), [plans]);
  useEffect(()=>{
    setSelectedPeriod(current=>current==="monthly"&&hasMonthly||current==="yearly"&&hasYearly?current:hasMonthly?"monthly":"yearly");
  },[hasMonthly,hasYearly]);
  const visiblePlans = prioritizeFreePlans(plansForBillingPeriod(plans, selectedPeriod));
  const recommendedId = visiblePlans.find(plan => plan.is_most_popular === true && Number(plan.price) > 0 && plan.pricing_type !== "custom" && plan.billing_period.toLowerCase() !== "custom")?.id;
  const currentPlan = plans.find(plan => plan.id === currentPlanId);

  async function choose(plan: PublicPlan) {
    if (plan.pricing_unavailable || choosingRef.current) return;
    choosingRef.current = true;
    setChoosing(plan.id);
    setActionError("");
    const params = new URLSearchParams({ plan: String(plan.id) });
    if (billingPeriod(plan) !== "other") params.set("billing_period", plan.billing_period.toLowerCase());
    const destination = planBillingDestination(plan.id, plan.billing_period)+
      "&"+new URLSearchParams({currency:plan.currency,reviewed_amount:plan.quote_total||String(plan.price)});
    params.set("currency",plan.currency);
    try {
      if (getUserToken()) {
        try { await apiRequest("/user/me"); }
        catch (reason) {
          if (getUserToken()) throw reason;
          router.push(customerLoginUrl(destination));
          return;
        }
        router.push(destination);
      } else {
        router.push(isFreePlan(plan) ? `/register?${params}` : customerLoginUrl(destination));
      }
    } catch (reason) {
      setActionError(localizeError(reason instanceof TypeError ? "Could not verify your customer session. Please check your connection and try again." : reason));
    } finally {
      choosingRef.current = false;
      setChoosing(null);
    }
  }

  if (content.enabled === false) return <><PublicHeader /><PublicFooter /></>;

  return <>
    <PublicHeader />
    <main dir={language === "ar" ? "rtl" : "ltr"} className="min-h-screen bg-gradient-to-br from-rose-50 via-fuchsia-50 to-violet-50 px-4 py-12 sm:px-6 sm:py-16">
      <div className="mx-auto max-w-7xl">
        <header className="mx-auto max-w-3xl text-center">
          <h1 className="text-3xl font-black tracking-tight text-slate-950 sm:text-4xl lg:text-5xl">{t("Choose the plan that fits you")}</h1>
          <p className="mx-auto mt-4 max-w-2xl text-base leading-7 text-slate-600 sm:text-lg">{t("Simple, flexible pricing for everyone.")}</p>
        </header>

        <CurrencyDropdown currency={currency} onChange={update} language={language}/>
        <div className="mt-8 flex justify-center">
          <div role="group" aria-label={t("Billing period")} className="inline-flex rounded-full border border-violet-200 bg-white p-1 shadow-sm">
            <button type="button" aria-pressed={selectedPeriod === "monthly"} disabled={!hasMonthly} onClick={() => setSelectedPeriod("monthly")} className={`min-h-11 rounded-full px-5 text-sm font-bold transition ${selectedPeriod === "monthly" ? "bg-violet-600 text-white shadow" : "text-slate-600 hover:bg-violet-50"} disabled:cursor-not-allowed disabled:opacity-45`}>{t("Monthly")}</button>
            <button type="button" aria-pressed={selectedPeriod === "yearly"} disabled={!hasYearly} onClick={() => setSelectedPeriod("yearly")} className={`flex min-h-11 items-center gap-2 rounded-full px-5 text-sm font-bold transition ${selectedPeriod === "yearly" ? "bg-violet-600 text-white shadow" : "text-slate-600 hover:bg-violet-50"} disabled:cursor-not-allowed disabled:opacity-45`}>
              {t("Yearly")}{yearlySavings > 0 && <span className={`rounded-full px-2 py-0.5 text-[10px] font-extrabold ${selectedPeriod === "yearly" ? "bg-white/20 text-white" : "bg-emerald-50 text-emerald-700"}`}>{t("Save up to {count}%", { count: yearlySavings })}</span>}
            </button>
          </div>
        </div>

        {loading && <p role="status" className="py-16 text-center text-slate-600">{t("Loading plans…")}</p>}
        {actionError && <p role="alert" className="mx-auto mt-6 max-w-2xl rounded-xl bg-red-50 p-4 text-center text-red-800">{actionError}</p>}
        {error && <div role="alert" className="mx-auto mt-10 max-w-2xl rounded-2xl border border-red-200 bg-white p-5 text-center text-red-700 shadow-sm"><p>{error}</p><button type="button" onClick={() => { setLoading(true); setError(""); setReloadPlans(value => value + 1); }} className="mt-3 rounded-lg bg-violet-600 px-4 py-2 font-semibold text-white">{t("Retry")}</button></div>}
        {!loading && !error && visiblePlans.length === 0 && <p className="py-12 text-center text-slate-600">{t("No {period} plans are available yet.", { period: t(selectedPeriod) })}</p>}
        {!loading && !error && visiblePlans.length > 0 && <div className="mt-9 grid auto-rows-fr gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {visiblePlans.map(plan => {
            const isCurrent = currentPlanId === plan.id && (!currentTerms.billing_period||currentTerms.billing_period===plan.billing_period)&&(!currentTerms.currency||currentTerms.currency===plan.currency)&& !(["trial", "trialing"].includes(subscriptionStatus) && !isFreePlan(plan));
            const custom = plan.pricing_type === "custom" || plan.billing_period.toLowerCase() === "custom";
            const actionLabel = isCurrent ? t("Current Plan") : custom ? t(plan.cta_text || "Contact Sales") : currentPlanId === plan.id && !isFreePlan(plan) ? t("Upgrade Plan") : currentPlan
              ? t(planChangeLabel(plan, currentPlan))
              : t(isFreePlan(plan) ? "Get Started" : "Upgrade Plan");
            const price = formatPlanPrice(plan, t, language);
            return <PlanPricingCard
              key={plan.id+":"+plan.billing_period}
              plan={plan}
              features={planFeatureLabels(plan, t)}
              price={price}
              popular={plan.id === recommendedId}
              current={isCurrent}
              actionLabel={choosing === plan.id ? t("Loading…") : actionLabel}
              disabled={choosing !== null}
              actionHref={custom ? plan.cta_url || "/contact?purpose=enterprise" : undefined}
              onAction={() => choose(plan)}
              t={t}
              language={language}
            />;
          })}
        </div>}
      </div>
    </main>
    <PublicFooter />
  </>;
}
