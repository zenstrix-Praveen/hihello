"use client";
import { useCustomerLocale } from "@/lib/customer-locale";
import { useUserData } from "./data";
import { LoadState } from "./Widgets";
export default function ResourceList({title,kind,columns}:{title:string;kind:string;columns:[string,string][]}) {
 const { t } = useCustomerLocale();
 

 const state=useUserData<{data:Record<string,string|number|null>[]}>(`/user/${kind}`);
 return <div className="space-y-6"><h1 className="text-3xl font-bold">{t(title)}</h1><p className="text-sm text-slate-500">{t("Your most recent records, up to 200.")}</p><LoadState {...state} retry={state.reload}/>{state.data && <div className="overflow-x-auto rounded-2xl border border-slate-200 bg-white">{state.data.data.length ? <table className="w-full text-start text-sm"><thead className="bg-slate-50 text-slate-500"><tr>{columns.map(([key,label])=><th className="px-5 py-4" key={key}>{t(label)}</th>)}</tr></thead><tbody className="divide-y divide-slate-100">{state.data.data.map(row=><tr key={row.id}>{columns.map(([key])=><td key={key} className="px-5 py-4">{["email","phone"].includes(key)?<bdi dir="ltr">{row[key] ?? "—"}</bdi>:key==="source"?t(String(row[key] ?? "—")):row[key] ?? "—"}</td>)}</tr>)}</tbody></table> : <p className="p-10 text-center text-slate-500">{t("No records yet.")}</p>}</div>}</div>;
}
