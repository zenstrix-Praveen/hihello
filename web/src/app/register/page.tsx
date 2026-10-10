"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import CustomerLanguageSwitcher from "@/components/locale/CustomerLanguageSwitcher";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { FormEvent, Suspense, useState } from "react";
import { apiRequest } from "@/lib/user-api";
import { Eye, EyeOff } from "lucide-react";

function RegisterForm() {
 const { t, error: localizeError } = useCustomerLocale();

  const router = useRouter();
  const params = useSearchParams();
  const template = params.get("template");
  const plan = params.get("plan");
  const billingPeriod = params.get("billing_period");
  const query = new URLSearchParams();
  if (template) query.set("template", template);
  if (plan) query.set("plan", plan);
  if (plan && (billingPeriod === "monthly" || billingPeriod === "yearly")) query.set("billing_period", billingPeriod);

  for(const key of ["currency","country","reviewed_amount"]){const value=params.get(key);if(plan&&value)query.set(key,value);}
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);

  async function register(event: FormEvent) {
    event.preventDefault();
    if (loading) return;
    setError("");

    if (!name.trim()) { setError("Full name is required."); return; }
if (!email.trim()) { setError("Email is required."); return; } if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) { setError("Enter a valid email address."); return; }
if (!password || !confirmPassword) { setError("Password and confirmation are required."); return; }
if (password !== confirmPassword) {
      setError("Passwords do not match.");
      return;
    }

    if (Array.from(password).length < 8) {
      setError("Password must contain at least 8 characters.");
      return;
    }

    setLoading(true);

    try {
      await apiRequest("/auth/register", {
        method: "POST",
        body: JSON.stringify({ name, email, phone, password }),
      });
      if (typeof window !== "undefined") {
        sessionStorage.setItem("auth_email", email.trim().toLowerCase());
        sessionStorage.setItem("auth_purpose", "customer_registration");
        sessionStorage.setItem("otp_resend_at", String(Date.now() + 60000));
        sessionStorage.setItem("auth_next", template ? `/dashboard/cards/new?template=${encodeURIComponent(template)}` : plan ? `/dashboard/billing?${query}` : "/dashboard");
      }
      router.push("/verify-otp?purpose=customer_registration");
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Could not create account."
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      

      <main className="relative min-h-[760px] bg-slate-50 px-5 py-16"><div className="absolute right-4 top-4 z-20"><CustomerLanguageSwitcher /></div>
        <div className="mx-auto max-w-xl rounded-3xl border bg-white p-8 shadow-sm sm:p-10">
          <div className="text-center">
            <p className="text-sm font-bold uppercase tracking-[0.2em] text-blue-600">{t("Get Started")}</p>

            <h1 className="mt-3 text-4xl font-black text-slate-950">{t("Create your account")}</h1>

            <p className="mt-3 text-slate-500">{t("Create your account and start building your digital business card.")}</p>
          </div>

          <form noValidate onSubmit={register} className="mt-8 space-y-5">
            <label className="block">
              <span className="text-sm font-semibold text-slate-700">{t("Full Name *")}</span>

              <input
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="mt-2 w-full rounded-xl border border-slate-300 px-4 py-3 outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
                placeholder={t("Your full name")}
              />
            </label>

            <label className="block">
              <span className="text-sm font-semibold text-slate-700">{t("Email *")}</span>

              <input
                type="email" dir="ltr"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="mt-2 w-full rounded-xl border border-slate-300 px-4 py-3 outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
                placeholder={t("you@example.com")}
              />
            </label>

            <label className="block">
              <span className="text-sm font-semibold text-slate-700">{t("Phone")}</span>

              <input
                type="tel" dir="ltr"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                className="mt-2 w-full rounded-xl border border-slate-300 px-4 py-3 outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
                placeholder={t("+91 98765 43210")}
              />
            </label>

            <div className="grid gap-5 sm:grid-cols-2">
              <label className="block">
                <span className="text-sm font-semibold text-slate-700">{t("Password *")}</span>

                <div className="relative mt-2"><input
                  dir="ltr" type={showPassword ? "text" : "password"}
                  required
                  minLength={8}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full rounded-xl border border-slate-300 px-4 py-3 pe-12 outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
                  autoComplete="new-password"
                />
                <button type="button" aria-label={showPassword ? t("Hide password") : t("Show password")} onClick={() => setShowPassword(!showPassword)} className="absolute inset-y-0 end-3 text-slate-500">{showPassword ? <EyeOff size={19}/> : <Eye size={19}/>}</button></div>
                <span className="mt-1 block text-xs text-slate-500">{t("Use at least 8 characters. A mix of letters, numbers, and symbols makes it stronger.")}</span>
              </label>

              <label className="block">
                <span className="text-sm font-semibold text-slate-700">{t("Confirm Password *")}</span>

                <div className="relative mt-2"><input
                  dir="ltr" type={showConfirm ? "text" : "password"}
                  required
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  className="w-full rounded-xl border border-slate-300 px-4 py-3 pe-12 outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-100"
                  autoComplete="new-password"
                />
                <button type="button" aria-label={showConfirm ? t("Hide password") : t("Show password")} onClick={() => setShowConfirm(!showConfirm)} className="absolute inset-y-0 end-3 text-slate-500">{showConfirm ? <EyeOff size={19}/> : <Eye size={19}/>}</button></div>
              </label>
            </div>

            {error && (
              <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700">
                {localizeError(error)}
              </div>
            )}

            <button
              type="submit"
              disabled={loading}
              className="w-full rounded-xl bg-blue-600 px-5 py-3.5 font-bold text-white transition hover:bg-blue-700 disabled:opacity-60"
            >
              {loading ? t("Creating Account...") : t("Create Account")}
            </button>
          </form>

          <p className="mt-6 text-center text-sm text-slate-500">{t("Already have an account?")}{" "}
            <Link href={query.size ? `/login?${query}` : "/login"} className="font-semibold text-blue-600">{t("Sign in")}</Link>
          </p>
        </div>
      </main>

      
    </>
  );
}

export default function RegisterPage() {
 const { t } = useCustomerLocale();

  return <Suspense fallback={<main className="relative p-8"><div className="absolute right-4 top-4 z-20"><CustomerLanguageSwitcher /></div>{t("Loading registration...")}</main>}><RegisterForm /></Suspense>;
}
