"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import CustomerLanguageSwitcher from "@/components/locale/CustomerLanguageSwitcher";
import Link from "next/link";
import BrandLogo from "@/components/brand/BrandLogo";
import {
  UserRound,
  ShieldCheck,
  ArrowRight,
} from "lucide-react";

export default function LoginChooserPage() {
 const { t } = useCustomerLocale();

  return (
    <main className="relative min-h-screen bg-slate-50 px-5 py-16"><div className="absolute right-4 top-4 z-20"><CustomerLanguageSwitcher /></div>
      <div className="mx-auto max-w-5xl">
        <div className="mb-10 flex justify-center"><Link href="/" aria-label={t("DBIZCARD home")}><BrandLogo className="h-12 w-[190px] max-w-full" priority /></Link></div>
        <div className="text-center">
          <p className="text-sm font-bold uppercase tracking-[0.25em] text-blue-600">{t("Welcome Back")}</p>

          <h1 className="mt-3 text-4xl font-black text-slate-950 sm:text-5xl">{t("Choose how you want to sign in")}</h1>

          <p className="mx-auto mt-4 max-w-2xl text-slate-500">{t("Select Customer Login to manage your digital cards, or Administrator Login to manage the platform.")}</p>
        </div>

        <div className="mt-12 grid gap-6 md:grid-cols-2">

          <Link
            href="/login"
            className="group rounded-3xl border border-slate-200 bg-white p-8 shadow-sm transition hover:-translate-y-1 hover:border-blue-300 hover:shadow-xl"
          >
            <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-blue-50 text-blue-600">
              <UserRound size={28} />
            </div>

            <h2 className="mt-6 text-2xl font-black text-slate-950">{t("Customer Login")}</h2>

            <p className="mt-3 leading-7 text-slate-500">{t("For customers managing digital business cards.")}</p>

            <div className="mt-8 flex items-center gap-2 font-bold text-blue-600">{t("Continue as Customer")}<ArrowRight
                size={18}
                className="transition rtl:rotate-180 group-hover:translate-x-1"
              />
            </div>
          </Link>

          <Link
            href="/admin/login"
            className="group rounded-3xl border border-slate-200 bg-white p-8 shadow-sm transition hover:-translate-y-1 hover:border-slate-400 hover:shadow-xl"
          >
            <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-slate-100 text-slate-800">
              <ShieldCheck size={28} />
            </div>

            <h2 className="mt-6 text-2xl font-black text-slate-950">{t("Admin Login")}</h2>

            <p className="mt-3 leading-7 text-slate-500">{t("For platform administrators.")}</p>

            <div className="mt-8 flex items-center gap-2 font-bold text-slate-800">{t("Continue as Admin")}<ArrowRight
                size={18}
                className="transition rtl:rotate-180 group-hover:translate-x-1"
              />
            </div>
          </Link>

        </div>

        <p className="mt-10 text-center text-sm text-slate-500">{t("New customer?")}{" "}
          <Link
            href="/register"
            className="font-bold text-blue-600"
          >{t("Create an account")}</Link>
        </p>
      </div>
    </main>
  );
}
