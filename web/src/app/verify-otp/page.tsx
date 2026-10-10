"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import CustomerLanguageSwitcher from "@/components/locale/CustomerLanguageSwitcher";import {Suspense} from "react";import CustomerOTPForm from "@/components/auth/CustomerOTPForm";
export default function VerifyOTPPage(){
 const { t } = useCustomerLocale();
return <><main className="relative grid min-h-[70vh] place-items-center bg-slate-50 px-4 py-12"><div className="absolute right-4 top-4 z-20"><CustomerLanguageSwitcher /></div><Suspense fallback={<p>{t("Loading…")}</p>}><CustomerOTPForm/></Suspense></main></>}
