"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import CustomerLanguageSwitcher from "@/components/locale/CustomerLanguageSwitcher";

import Link from "next/link";
import BrandLogo from "@/components/brand/BrandLogo";
import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Eye, EyeOff } from "lucide-react";
import { customerLoginDestination, customerRegistrationUrl } from "@/lib/customer-upgrade";

import {
  apiRequest,
  getUserToken,
  saveUserToken,
} from "@/lib/user-api";

export default function CustomerLoginPage() {
 const { t, error: localizeError } = useCustomerLocale();

  const router = useRouter();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [returnTo, setReturnTo] = useState("/dashboard");

  useEffect(() => {
    setReturnTo(customerLoginDestination(window.location?.search || ""));
    if (!getUserToken()) return;
    let active = true;
    apiRequest("/user/me")
      .then(() => { if (active) router.replace(customerLoginDestination(window.location?.search || "")); })
      .catch(() => { /* An invalid customer token is cleared by apiRequest. */ });
    return () => { active = false; };
  }, [router]);

  async function submit(event: FormEvent) {
    event.preventDefault();
 if (loading) return;
if (!email.trim()) { setError("Email is required."); return; } if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) { setError("Enter a valid email address."); return; }
 if (!password) { setError("Password is required."); return; }

    setLoading(true);
    setError("");

    try {
      const data = await apiRequest("/auth/login", {
        method: "POST",
        body: JSON.stringify({
          email,
          password,
        }),
      });

      if (!data?.token) {
        throw new Error("Login did not return a valid customer session.");
      }

      saveUserToken(data.token);

      router.replace(customerLoginDestination(window.location?.search || ""));
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Invalid email or password"
      );
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="relative min-h-screen bg-slate-50 px-5 py-10 sm:py-16"><div className="absolute right-4 top-4 z-20"><CustomerLanguageSwitcher /></div>
      <div className="mb-8 flex justify-center"><Link href="/" aria-label={t("DBIZCARD home")}><BrandLogo className="h-12 w-[190px] max-w-full" priority /></Link></div>
      <div className="mx-auto max-w-md rounded-3xl border border-slate-200 bg-white p-8 shadow-xl">

        <div className="text-center">
          <p className="text-sm font-bold uppercase tracking-[0.22em] text-blue-600">{t("Customer Access")}</p>

          <h1 className="mt-3 text-3xl font-black text-slate-950">{t("Customer Login")}</h1>

          <p className="mt-2 text-sm text-slate-500">{t("Sign in to manage your digital cards and account.")}</p>
        </div>

        <form noValidate onSubmit={submit} className="mt-8 space-y-5">

          <label className="block">
            <span className="text-sm font-semibold text-slate-700">{t("Email")}</span>

            <input
              type="email" dir="ltr"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="mt-2 w-full rounded-xl border border-slate-300 px-4 py-3"
            />
          </label>

          <label className="block">
            <span className="text-sm font-semibold text-slate-700">{t("Password")}</span>

            <div className="relative mt-2">
              <input
                dir="ltr" type={showPassword ? "text" : "password"}
                required
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="w-full rounded-xl border border-slate-300 px-4 py-3 pe-12"
              />

              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute inset-y-0 end-3 text-slate-500"
                aria-label={
                  showPassword
                    ? t("Hide password")
                    : t("Show password")
                }
              >
                {showPassword ? (
                  <EyeOff size={19} />
                ) : (
                  <Eye size={19} />
                )}
              </button>
            </div>
          </label>

          <div className="text-end">
            <Link
              href="/forgot-password"
              className="text-sm font-semibold text-blue-600"
            >{t("Forgot Password?")}</Link>
          </div>

          {error && (
            <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700">
              {localizeError(error)}
            </div>
          )}

          <button
            disabled={loading}
            className="w-full rounded-xl bg-blue-600 py-3.5 font-bold text-white hover:bg-blue-700 disabled:opacity-60"
          >
            {loading ? t("Signing in...") : t("Sign In")}
          </button>
        </form>

        <div className="mt-7 space-y-3 text-center text-sm">
          <p className="text-slate-500">{t("Don't have an account?")}{" "}
            <Link
              href={customerRegistrationUrl(returnTo)}
              className="font-bold text-blue-600"
            >{t("Create Account")}</Link>
          </p>

          <Link
            href="/signin"
            className="block font-semibold text-slate-600"
          >{t("Sign in another way")}</Link>
        </div>
      </div>
    </main>
  );
}
