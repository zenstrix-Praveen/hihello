"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import {
  ArrowUpRight,
  ChevronDown,
  CreditCard,
  LogOut,
  Menu,
  Plus,
  Sparkles,
  UserRound,
} from "lucide-react";
import { useUser } from "./UserSession";
import { logoutUser } from "@/lib/user-api";
import { useCustomerLocale } from "@/lib/customer-locale";
import CustomerLanguageSwitcher from "@/components/locale/CustomerLanguageSwitcher";

export default function UserHeader({ onMenu, menuOpen }: { onMenu: () => void; menuOpen: boolean }) {
  const { t } = useCustomerLocale();
  const user = useUser();
  const router = useRouter();
  const pathname = usePathname();
  const [profileOpen, setProfileOpen] = useState(false);
  const profileMenuRef = useRef<HTMLDivElement>(null);
  const profileButtonRef = useRef<HTMLButtonElement>(null);
  const name = user.name?.trim() || t("Customer");
  const initials = name.split(/\s+/).filter(Boolean).slice(0, 2).map((part) => part[0]).join("").toUpperCase();

  useEffect(() => {
    if (!profileOpen) return;
    const closeOnOutsideClick = (event: PointerEvent) => {
      if (event.target instanceof Node && !profileMenuRef.current?.contains(event.target)) setProfileOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setProfileOpen(false);
        profileButtonRef.current?.focus();
      }
    };
    document.addEventListener("pointerdown", closeOnOutsideClick);
    document.addEventListener("keydown", closeOnEscape);
    profileMenuRef.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsideClick);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [profileOpen]);

  useEffect(() => setProfileOpen(false), [pathname]);

  function handleMenuKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    const items = Array.from(profileMenuRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []);
    const currentIndex = items.indexOf(document.activeElement as HTMLElement);
    let nextIndex: number | undefined;
    if (event.key === "ArrowDown") nextIndex = (currentIndex + 1) % items.length;
    if (event.key === "ArrowUp") nextIndex = (currentIndex - 1 + items.length) % items.length;
    if (event.key === "Home") nextIndex = 0;
    if (event.key === "End") nextIndex = items.length - 1;
    if (nextIndex !== undefined && items.length > 0) {
      event.preventDefault();
      items[nextIndex]?.focus();
    }
  }

  function handleLogout() {
    setProfileOpen(false);
    logoutUser();
    router.replace("/login");
  }

  return (
    <header className="sticky top-0 z-30 flex h-20 items-center justify-between gap-3 border-b border-slate-200 bg-white/95 px-4 backdrop-blur sm:px-8">
      <div className="flex min-w-0 items-center gap-3">
        <button type="button" onClick={() => { window.dispatchEvent(new Event("dbizcard:close-chat-request")); onMenu(); }} aria-label={t(menuOpen ? "Close navigation" : "Open navigation")} aria-expanded={menuOpen} aria-controls="customer-sidebar" className="grid h-11 w-11 shrink-0 place-items-center rounded-xl border border-slate-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 lg:hidden">
          <Menu size={22} />
        </button>
        <span className="truncate text-sm font-semibold text-slate-500">{t("Customer workspace")}</span>
      </div>

      <div className="flex shrink-0 items-center gap-3 sm:gap-5">
        <Link href="/dashboard/cards/new" className="hidden items-center gap-2 text-sm font-semibold text-blue-600 focus-visible:rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 sm:flex">
          <Plus size={17} aria-hidden="true" />{t("Create card")}
        </Link>

        <CustomerLanguageSwitcher />
        <div ref={profileMenuRef} className="relative">
          <button
            ref={profileButtonRef}
            type="button"
            onClick={() => setProfileOpen((open) => !open)}
            aria-label={t("Open account menu for {name}", {name})}
            aria-haspopup="menu"
            aria-expanded={profileOpen}
            aria-controls="customer-profile-menu"
            className="flex max-w-[calc(100vw-7rem)] items-center gap-2 rounded-xl p-1.5 text-start transition hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 sm:gap-3 sm:px-2"
          >
            <span className="hidden max-w-56 text-end sm:block">
              <span className="block truncate text-sm font-bold text-slate-900">{name}</span>
              <span className="block truncate text-xs text-slate-500"><bdi dir="ltr">{user.email}</bdi></span>
            </span>
            <span aria-hidden="true" className="grid h-10 w-10 shrink-0 place-items-center rounded-full bg-blue-100 text-sm font-bold uppercase text-blue-700 ring-1 ring-blue-200">{initials || "C"}</span>
            <ChevronDown aria-hidden="true" size={16} className={`hidden text-slate-400 transition-transform sm:block ${profileOpen ? "rotate-180" : ""}`} />
          </button>

          {profileOpen && (
            <div className="fixed left-4 right-4 top-[4.75rem] z-[60] overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-xl shadow-slate-900/15 sm:absolute sm:start-auto sm:end-0 sm:top-full sm:mt-2 sm:w-[320px]">
              <div className="flex items-center gap-3 border-b border-slate-100 bg-slate-50/80 p-4">
                <span aria-hidden="true" className="grid h-12 w-12 shrink-0 place-items-center rounded-full bg-blue-100 text-base font-bold uppercase text-blue-700">{initials || "C"}</span>
                <div className="min-w-0">
                  <p className="truncate text-sm font-bold text-slate-900">{name}</p>
                  <p className="truncate text-xs text-slate-500"><bdi dir="ltr">{user.email}</bdi></p>
                </div>
              </div>

              <div id="customer-profile-menu" role="menu" aria-label={t("Customer account")} onKeyDown={handleMenuKeyDown} className="p-2">
                <Link role="menuitem" tabIndex={-1} href="/dashboard/profile" onClick={() => setProfileOpen(false)} className="flex min-h-11 items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-700 transition hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">
                  <UserRound size={18} className="text-slate-500" aria-hidden="true" />{t("Account")}
                </Link>
                <Link role="menuitem" tabIndex={-1} href="/dashboard/billing" onClick={() => setProfileOpen(false)} className="flex min-h-11 items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-slate-700 transition hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">
                  <CreditCard size={18} className="text-slate-500" aria-hidden="true" />{t("Subscription")}
                </Link>
                <Link role="menuitem" tabIndex={-1} href="/dashboard/billing" onClick={() => setProfileOpen(false)} className="mt-1 flex min-h-11 items-center gap-3 rounded-xl bg-blue-50 px-3 py-2.5 text-sm font-semibold text-blue-700 transition hover:bg-blue-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">
                  <Sparkles size={18} aria-hidden="true" />{t("Upgrade Plan")}<ArrowUpRight size={15} className="ms-auto" aria-hidden="true" />
                </Link>
              </div>

              <div className="border-t border-slate-100 p-2">
                <button role="menuitem" tabIndex={-1} type="button" onClick={handleLogout} className="flex min-h-11 w-full items-center gap-3 rounded-xl px-3 py-2.5 text-start text-sm font-medium text-slate-600 transition hover:bg-red-50 hover:text-red-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500">
                  <LogOut size={18} aria-hidden="true" />{t("Logout")}
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
