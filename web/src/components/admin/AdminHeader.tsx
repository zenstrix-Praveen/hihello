"use client";
import {Bell,Globe2,LogOut,Menu,Search} from "lucide-react";
import {FormEvent,useEffect,useState} from "react";
import Link from "next/link";
import {useRouter} from "next/navigation";
import {adminApi} from "@/lib/admin-api";
import {useAdminLocale} from "@/lib/admin-locale";
type AdminProfile={name?:string;email?:string;role?:string};
export default function AdminHeader({onMenu,menuOpen}:{onMenu:()=>void;menuOpen:boolean}){
 const router=useRouter();
 const {language,languages,setLanguage,t}=useAdminLocale();
 const [profile,setProfile]=useState<AdminProfile>({});
 const [search,setSearch]=useState("");
 useEffect(()=>{let mounted=true;try{const cached=JSON.parse(localStorage.getItem("admin_user")||"{}");if(mounted)setProfile(cached)}catch{}
  adminApi("/admin/auth/me").then(x=>{if(mounted){setProfile(x.data||{});localStorage.setItem("admin_user",JSON.stringify(x.data||{}))}}).catch(()=>{});
  return()=>{mounted=false}},[]);
 const name=profile.name||profile.email||"Admin";const initials=name.split(/[\s@._-]+/).filter(Boolean).slice(0,2).map(x=>x[0].toUpperCase()).join("")||"A";
 function logout(){localStorage.removeItem("admin_token");localStorage.removeItem("admin_user");router.replace("/admin/login");router.refresh()}
 function submitSearch(e:FormEvent){e.preventDefault();const q=search.trim();if(q){const href=`/admin/search?q=${encodeURIComponent(q)}`;router.push(href);window.dispatchEvent(new CustomEvent("admin:navigate",{detail:href}))}}
 return <header className="sticky top-0 z-30 flex h-16 items-center justify-between gap-2 border-b bg-white px-2 sm:h-20 sm:gap-3 sm:px-6 lg:px-8"><div className="flex min-w-0 flex-1 items-center gap-1.5 sm:flex-none sm:gap-2"><button type="button" onClick={onMenu} aria-label={menuOpen?"Close navigation":"Open navigation"} aria-expanded={menuOpen} className="grid h-9 w-9 shrink-0 place-items-center rounded-xl border text-slate-700 sm:h-10 sm:w-10 lg:hidden"><Menu size={20}/></button><form onSubmit={submitSearch} role="search" className="relative w-full max-w-[25vw] sm:w-80 sm:max-w-[45vw]"><button type="submit" aria-label={t("Search")} className="absolute left-0 top-0 grid h-10 w-9 place-items-center text-slate-400"><Search size={18}/></button><input aria-label={t("Search users, cards, organizations...")} value={search} onChange={e=>setSearch(e.target.value)} placeholder={t("Search...")} className="h-10 w-full rounded-xl bg-slate-100 pl-9 pr-1 text-xs outline-none sm:pr-4 sm:text-sm"/></form></div><div className="flex shrink-0 items-center gap-0.5 sm:gap-3"><label className="flex h-9 items-center gap-0.5 rounded-lg border px-1 text-sm text-slate-700 sm:h-10 sm:gap-2 sm:rounded-xl sm:px-3"><Globe2 size={15} className="shrink-0 text-slate-500"/><span className="sr-only">{t("Select language")}</span><select aria-label={t("Select language")} value={language} onChange={e=>setLanguage(e.target.value)} className="max-w-[3.4rem] bg-transparent text-[10px] outline-none sm:max-w-28 sm:text-sm">{languages.map(x=><option key={x.code} value={x.code}>{x.name}</option>)}</select></label><Link href="/admin/notifications" aria-label={t("Notifications")} className="grid h-9 w-9 place-items-center rounded-lg border sm:h-10 sm:w-10 sm:rounded-xl"><Bell size={17}/></Link><Link href="/admin/settings/profile" className="flex items-center gap-3 rounded-xl px-0.5 py-1 hover:bg-slate-50 sm:px-1" aria-label={t("Profile")}><span className="hidden max-w-44 text-right sm:block"><span className="block truncate text-sm font-semibold">{name}</span><span className="block truncate text-xs text-slate-500">{profile.email||profile.role||""}</span></span><span className="grid h-9 w-9 place-items-center rounded-lg bg-blue-600 text-sm font-bold text-white sm:h-10 sm:w-10 sm:rounded-xl" aria-hidden="true">{initials}</span></Link><button onClick={logout} title={t("Logout")} aria-label={t("Logout")} className="grid h-9 w-9 place-items-center rounded-lg border text-slate-600 hover:bg-slate-50 sm:h-10 sm:w-10 sm:rounded-xl"><LogOut size={16}/></button></div></header>
}

