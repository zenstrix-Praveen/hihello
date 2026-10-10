"use client";

import { useCustomerLocale } from "@/lib/customer-locale";

/** A route render failure should remain visible and recoverable instead of looking like an empty page. */
export default function RouteError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  const { t } = useCustomerLocale();

  return (
    <main role="alert" className="grid min-h-[60vh] place-items-center bg-slate-50 px-5 py-16 text-center">
      <section className="max-w-lg rounded-2xl border border-slate-200 bg-white p-8 shadow-sm">
        <h1 className="text-2xl font-bold text-slate-900">{t("Something went wrong. Please try again.")}</h1>
        <p className="mt-3 text-sm leading-6 text-slate-600">{t("If the problem continues, refresh the page or contact support.")}</p>
        <button type="button" onClick={reset} className="mt-6 min-h-11 rounded-xl bg-blue-600 px-5 font-semibold text-white hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2">
          {t("Retry")}
        </button>
      </section>
    </main>
  );
}
