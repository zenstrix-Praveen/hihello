"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { LayoutDashboard, ContactRound, Plus, PanelsTopLeft, Users, QrCode, SmartphoneNfc, ChartNoAxesCombined, CreditCard, CircleUserRound, LogOut, X } from "lucide-react";
import { logoutUser } from "@/lib/user-api";
import { useCustomerLocale } from "@/lib/customer-locale";
import BrandLogo from "@/components/brand/BrandLogo";
const items = [
  ["Dashboard", "/dashboard", LayoutDashboard], ["My Digital Cards", "/dashboard/cards", ContactRound],
  ["Create Card", "/dashboard/cards/new", Plus], ["Templates", "/dashboard/templates", PanelsTopLeft],
  ["Contacts", "/dashboard/contacts", Users], ["QR Codes", "/dashboard/qr-codes", QrCode],
  ["NFC", "/dashboard/nfc", SmartphoneNfc], ["Analytics", "/dashboard/analytics", ChartNoAxesCombined],
  ["Plans & Billing", "/dashboard/billing", CreditCard], ["Profile", "/dashboard/profile", CircleUserRound],
] as const;
export default function UserSidebar({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t, language } = useCustomerLocale();
  const path = usePathname(); const router = useRouter();
  return <>
    {open && <button aria-label={t("Close menu overlay")} onClick={onClose} className="fixed inset-x-0 top-20 bottom-0 z-40 bg-slate-950/40 lg:hidden" />}
    <aside id="customer-sidebar" className={`fixed top-20 bottom-0 start-0 z-50 flex w-64 flex-col border-e border-slate-200 bg-white transition-transform lg:inset-y-0 lg:translate-x-0 ${open ? "translate-x-0" : language === "ar" ? "translate-x-full" : "-translate-x-full"}`}>
      <div className="flex min-h-20 items-center justify-between gap-2 border-b border-slate-100 px-4 py-2"><Link href="/dashboard" onClick={onClose} aria-label={t("DBIZCARD dashboard home")} className="min-w-0"><BrandLogo className="h-12 w-[190px] max-w-full" /><span className="mt-1 block ps-1 text-[10px] font-semibold uppercase tracking-[.2em] text-slate-400">{t("Your digital workspace")}</span></Link><button onClick={onClose} aria-label={t("Close menu")} className="rounded-lg p-2 lg:hidden"><X size={20} /></button></div>
      <nav aria-label={t("Customer navigation")} className="flex-1 space-y-1 overflow-y-auto px-3 py-5">{items.map(([label, href, Icon]) => {
        const active = path === href || (href === "/dashboard/cards" && path.startsWith(href + "/") && path !== "/dashboard/cards/new");
        return <Link key={href} href={href} onClick={onClose} aria-current={active ? "page" : undefined} className={`flex items-center gap-3 rounded-xl px-4 py-3 text-sm font-semibold transition ${active ? "bg-blue-50 text-blue-700" : "text-slate-600 hover:bg-slate-50 hover:text-slate-950"}`}><Icon size={19} />{t(label)}</Link>;
      })}</nav>
      <div className="border-t border-slate-100 p-4"><button onClick={() => { logoutUser(); router.replace("/login"); }} className="flex w-full items-center gap-3 rounded-xl px-4 py-3 text-sm font-semibold text-slate-600 hover:bg-slate-50"><LogOut size={19} />{t("Logout")}</button></div>
    </aside>
  </>;
}

