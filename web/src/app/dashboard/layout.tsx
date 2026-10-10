"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { apiRequest, getUserToken } from "@/lib/user-api";
import { customerLoginUrl } from "@/lib/customer-upgrade";
import UserSidebar from "@/components/dashboard/UserSidebar";
import UserHeader from "@/components/dashboard/UserHeader";
import { UserContext, UserProfile } from "@/components/dashboard/UserSession";
import { ChatbotLauncher } from "@/components/public/Chatbot";
export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const { t, error: localizeError } = useCustomerLocale();
  const router = useRouter();
  const [user, setUser] = useState<UserProfile | null>(null);
  const [error, setError] = useState(""); const [open, setOpen] = useState(false); const [retry, setRetry] = useState(0);
  useEffect(() => {
    let live = true;
    let validation = 0;
    const handleUnauthorized = () => {
      validation++;
      setUser(null);
      setError("");
      const destination = window.location?.pathname === "/dashboard/billing" ? `${window.location.pathname}${window.location.search}` : null;
      router.replace(destination ? customerLoginUrl(destination) : "/login");
    };
    window.addEventListener("customer:unauthorized", handleUnauthorized);
    window.addEventListener("customer:logout", handleUnauthorized);
    const validate = () => {
      const token = getUserToken();
      if (!token) { handleUnauthorized(); return; }
      const request = ++validation;
      const current = () => live && request === validation && getUserToken() === token;
      apiRequest("/user/me").then(data => { if (current()) { setUser(data); setError(""); } }).catch(e => { if (current()) setError(e.message); });
    };
    validate(); const timer = window.setInterval(validate, 60000);
    window.addEventListener("focus", validate); window.addEventListener("storage", validate);
    return () => { live = false; clearInterval(timer); window.removeEventListener("focus", validate); window.removeEventListener("storage", validate); window.removeEventListener("customer:unauthorized", handleUnauthorized); window.removeEventListener("customer:logout", handleUnauthorized); };
  }, [router, retry]);
  useEffect(() => {
    if (!open) return;
    const close = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    window.addEventListener("keydown", close); return () => window.removeEventListener("keydown", close);
  }, [open]);
  useEffect(() => {
    const closeNavigation = () => setOpen(false);
    const closeChat = () => window.dispatchEvent(new Event("dbizcard:close-chat"));
    window.addEventListener("dbizcard:close-navigation", closeNavigation);
    window.addEventListener("dbizcard:close-chat-request", closeChat);
    return () => {
      window.removeEventListener("dbizcard:close-navigation", closeNavigation);
      window.removeEventListener("dbizcard:close-chat-request", closeChat);
    };
  }, []);
  if (!user) return <main className="grid min-h-screen place-items-center bg-slate-50 p-6"><div role="status" className="max-w-md rounded-2xl border bg-white p-8 text-center text-slate-600">{error ? <><p>{localizeError(error)}</p><button className="mt-4 font-semibold text-blue-600" onClick={() => { setError(""); setRetry(x => x + 1); }}>{t("Try again")}</button></> : t("Loading your workspace…")}</div></main>;
  return <UserContext.Provider value={user}><div className="min-h-screen bg-slate-50 text-slate-900"><UserSidebar open={open} onClose={() => setOpen(false)} /><div className="min-w-0 lg:ps-64"><UserHeader onMenu={() => setOpen(value => !value)} menuOpen={open} /><main className="mx-auto max-w-[1500px] p-4 sm:p-8">{children}</main></div><ChatbotLauncher audience="customer" customerName={user.name} /></div></UserContext.Provider>;
}
