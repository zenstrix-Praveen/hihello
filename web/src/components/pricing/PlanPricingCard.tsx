import Link from "next/link";
import { ArrowRight, Check } from "lucide-react";
import { PublicPlan, planUsageLines } from "@/lib/pricing-plans";

type Props = {
  plan: PublicPlan;
  features: string[];
  price: { amount: string; period: string };
  popular?: boolean;
  current?: boolean;
  actionLabel: string;
  actionHref?: string;
  onAction?: () => void;
  onViewDetails?: () => void;
  disabled?: boolean;
  disabledReason?: string;
  t: (value: string, params?: Record<string, string | number>) => string;
  language: string;
};

export default function PlanPricingCard({ plan, features, price, popular = false, current = false, actionLabel, actionHref, onAction, onViewDetails, disabled = false, disabledReason, t, language }: Props) {
  disabled = disabled || Boolean(plan.pricing_unavailable);
  disabledReason = plan.pricing_unavailable ? (language === "ar" ? "لا يوجد سعر لهذه العملة والفترة. اختر عملة أخرى أو تواصل مع الدعم." : "No price is configured for this currency and period. Select another currency or contact support.") : disabledReason;
  const trial = plan.trial_mode === "unlimited" ? t("Unlimited Trial")
    : (plan.trial_mode === "limited" || (!plan.trial_mode && plan.trial_days > 0)) && plan.trial_days > 0
      ? t("{count} day trial", { count: plan.trial_days })
      : t("No trial");
  const billingType = plan.billing_type === "per_workspace" ? t("Per Workspace") : t("Per User");
  const subscriptionCondition = plan.billing_period.toLowerCase() === "lifetime" ? t("One-time payment")
    : plan.billing_period.toLowerCase() === "custom" ? t("Contact sales for subscription terms")
      : t("Payment covers {period}. Renewal requires another payment.", { period: t(plan.billing_period.toLowerCase()) });
  const ctaClass = `flex h-12 w-full items-center justify-center gap-2 rounded-full px-4 font-bold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-600 focus-visible:ring-offset-2 ${current || disabled ? "cursor-not-allowed border border-slate-200 bg-slate-100 text-slate-500" : price.amount === t("Free") ? "border border-violet-300 bg-white text-violet-700 hover:bg-violet-50" : "bg-gradient-to-r from-violet-600 to-purple-600 text-white shadow-sm hover:from-violet-700 hover:to-purple-700 hover:shadow-md"}`;

  const action = current || disabled ? (
    <button type="button" disabled className={ctaClass} aria-disabled="true">{current ? t("Current Plan") : actionLabel}</button>
  ) : actionHref ? (
    <Link href={actionHref} className={ctaClass}>{actionLabel}<ArrowRight aria-hidden="true" className={`h-4 w-4 ${language === "ar" ? "rotate-180" : ""}`} /></Link>
  ) : (
    <button type="button" onClick={onAction} className={ctaClass}>{actionLabel}<ArrowRight aria-hidden="true" className={`h-4 w-4 ${language === "ar" ? "rotate-180" : ""}`} /></button>
  );

  return (
    <article data-pricing-card className={`relative flex h-full min-w-0 flex-col rounded-3xl border bg-white p-6 shadow-[0_12px_36px_-24px_rgba(15,23,42,0.3)] transition-shadow duration-200 hover:shadow-[0_22px_48px_-26px_rgba(79,70,229,0.25)] ${popular ? "border-violet-400 shadow-[0_18px_48px_-26px_rgba(124,58,237,0.38)] ring-1 ring-violet-200" : "border-slate-200/80"}`}>
      <div className="min-h-16">
        <h2 className="text-xl font-bold tracking-tight text-slate-900">{t(plan.name)}</h2>
        {plan.description && <p className="mt-2 line-clamp-2 text-sm leading-5 text-slate-600">{plan.description}</p>}
        {popular && <span className="mt-3 inline-flex rounded-full bg-violet-600 px-3 py-1 text-[11px] font-extrabold uppercase tracking-wide text-white shadow-sm">{t("Most Popular")}</span>}
      </div>
      <div className="mt-4 border-t border-slate-200" />

      <section className="mt-5 flex-1" aria-label={t("What's included")}>
        <h3 className="text-xs font-extrabold uppercase tracking-[0.15em] text-slate-500">{t("What's included")}</h3>
        {features.length ? <ul className="mt-4 space-y-3">
          {features.map(feature => <li key={feature} className="flex min-w-0 items-start gap-2.5 text-sm leading-5 text-slate-700">
            <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-violet-50 text-violet-700"><Check aria-hidden="true" className="h-3.5 w-3.5" /></span>
            <span className="break-words">{feature}</span>
          </li>)}
        </ul> : <p className="mt-4 text-sm text-slate-500">{t("No additional features configured")}</p>}
      </section>

      <div data-pricing-bottom className="mt-auto pt-7">
        <div className="border-t border-slate-200 pb-5" />
        <div className="mb-5 min-h-[5rem] text-center">
          <p className="mb-1 text-xs font-extrabold uppercase tracking-[0.15em] text-slate-500">{t("Price")}</p>
          <p className="break-words text-3xl font-black tracking-tight text-slate-950 sm:text-4xl">{price.amount}</p>
          {price.period && <p className="mt-1 text-sm font-medium text-slate-500">{price.period}</p>}
        </div>
        {plan.quote_total&&<p className="mb-3 text-xs text-slate-600">{language==="ar"?"المجموع الفرعي":"Subtotal"}: {plan.currency} {plan.subtotal} · {language==="ar"?"الضريبة":"Tax"}: {plan.currency} {plan.tax} · {language==="ar"?"الإجمالي":"Total"}: {plan.currency} {plan.quote_total}</p>}
        {action}
        {disabledReason && !current && <p className="mt-2 text-center text-xs text-amber-800">{disabledReason}</p>}
        {onViewDetails ? <button type="button" onClick={onViewDetails} className="mt-3 min-h-8 w-full rounded px-2 py-1 text-center text-xs font-semibold text-violet-700 underline decoration-violet-200 underline-offset-4 hover:text-violet-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-600">{t("View Details")}</button> : <details className="group mt-3 text-center">
          <summary className="inline-flex min-h-8 cursor-pointer list-none items-center justify-center rounded px-2 py-1 text-xs font-semibold text-violet-700 underline decoration-violet-200 underline-offset-4 hover:text-violet-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-violet-600">{t("View Details")}</summary>
          <div className="mt-2 rounded-xl bg-slate-50 p-4 text-start text-xs leading-5 text-slate-600">
            <h4 className="font-bold text-slate-800">{t(plan.name)}</h4>
            {plan.description && <p className="mt-1 whitespace-pre-line">{plan.description}</p>}
            <p className="mt-2"><strong>{t("Price")}:</strong> {price.amount}{price.period ? ` · ${price.period}` : ""}</p>
            <p className="mt-1"><strong>{t("Trial availability")}:</strong> {trial}</p>
            <p className="mt-1"><strong>{t("Subscription terms")}:</strong> {subscriptionCondition}</p>
            <p className="mt-1"><strong>{t("Billing type")}:</strong> {billingType}</p>
            <h5 className="mt-3 font-bold text-slate-800">{t("Included features and limits")}</h5>
            {features.length ? <ul className="mt-1 list-inside list-disc space-y-0.5">{features.map(feature => <li key={feature}>{feature}</li>)}</ul> : <p className="mt-1">{t("No additional features configured")}</p>}
            {planUsageLines(plan, t).length > 0 && <>
              <h5 className="mt-3 font-bold text-slate-800">{t("Usage limits")}</h5>
              <ul className="mt-1 list-inside list-disc space-y-0.5">{planUsageLines(plan, t).map(limit => <li key={limit}>{limit}</li>)}</ul>
            </>}
          </div>
        </details>}
      </div>
    </article>
  );
}
