"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useAdminLocation } from "@/lib/admin-location";
import { adminApi } from "@/lib/admin-api";

type Item = { id: number; name: string; email?: string; status?: string; slug?: string; company_name?: string; owner_name?: string; organization_name?: string };

export default function AdminSearchPage() {
  const { search } = useAdminLocation();
  const query = new URLSearchParams(search).get("q") ?? "";
  const [users, setUsers] = useState<Item[]>([]);
  const [cards, setCards] = useState<Item[]>([]);
  const [organizations, setOrganizations] = useState<Item[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    if (!query) { setUsers([]); setCards([]); setOrganizations([]); return; }
    setLoading(true);
    setError("");
    try {
      const [userResult, cardResult, organizationResult] = await Promise.all([
        adminApi(`/admin/users?search=${encodeURIComponent(query)}&page=1&limit=20`),
        adminApi(`/admin/cards?search=${encodeURIComponent(query)}&page=1&limit=20`),
        adminApi(`/admin/organizations?search=${encodeURIComponent(query)}&page=1&limit=20`),
      ]);
      setUsers(userResult.data ?? []);
      setCards(cardResult.data ?? []);
      setOrganizations(organizationResult.data ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not search admin records");
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => { void load(); }, [load]);

  const groups: [string, Item[], (item: Item) => string][] = [
    ["Users", users, item => `/admin/users?search=${encodeURIComponent(item.email || item.name)}`],
    ["Digital Cards", cards, item => `/admin/cards?search=${encodeURIComponent(item.name)}`],
    ["Organizations", organizations, item => `/admin/organizations?search=${encodeURIComponent(item.name)}`],
  ];

  return <section className="max-w-6xl">
    <h1 className="text-3xl font-bold">Search</h1>
    <p className="mt-2 text-sm text-slate-500">Results for “{query}” across users, cards and organizations.</p>
    {error && <div role="alert" className="mt-5 rounded-xl bg-red-50 p-4 text-red-700">{error}<button onClick={() => void load()} className="ml-3 underline">Retry</button></div>}
    {!query ? <div className="mt-5 rounded-2xl border bg-white p-8 text-sm text-slate-500">Enter a name, email or card in the admin header search.</div> : loading ? <div className="mt-5 rounded-2xl border bg-white p-10 text-center text-slate-500">Searching…</div> : <div className="mt-5 space-y-5">
      {groups.map(([label, items, href]) => <section key={label} className="rounded-2xl border bg-white p-5">
        <h2 className="font-semibold">{label} ({items.length})</h2>
        {items.length === 0 ? <p className="mt-3 text-sm text-slate-500">No matching {label.toLowerCase()}.</p> : <ul className="mt-3 divide-y">
          {items.map(item => <li key={item.id}><Link href={href(item)} className="flex flex-wrap justify-between gap-2 py-3 hover:text-blue-700"><span><span className="font-medium">{item.name}</span><span className="ml-2 text-sm text-slate-500">{item.email || item.owner_name || item.company_name || item.organization_name || item.slug}</span></span><span className="text-sm capitalize text-slate-500">{item.status}</span></Link></li>)}
        </ul>}
      </section>)}
    </div>}
  </section>;
}
