"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { adminApi } from "@/lib/admin-api";
import NumericStepper from "@/components/admin/NumericStepper";
import {
  FeatureDefinition,
  EntitlementValues,
  PlanEntitlement,
  groupFeatures,
  saveAndReloadEntitlements,
  validateEntitlementValues,
  valuesFromEntitlements,
} from "@/lib/plan-entitlements";

type Props = { planId: number; planName: string; onClose: () => void; onSaved?: (entitlements: Record<string, PlanEntitlement>, registry: FeatureDefinition[]) => void };

export function PlanEntitlementsEditor({ planId, planName, onClose, onSaved }: Props) {
  const [registry, setRegistry] = useState<FeatureDefinition[]>([]);
  const [values, setValues] = useState<EntitlementValues>({});
  const [loading, setLoading] = useState(true);
  const [ready, setReady] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [limitedValues, setLimitedValues] = useState<Record<string,string>>({});

  const reload = useCallback(async (): Promise<boolean> => {
    setLoading(true);
    setReady(false);
    setError("");
    setRegistry([]);
    setValues({});
    try {
      const [registryResult, planResult] = await Promise.allSettled([
        adminApi("/admin/plans/feature-registry"),
        Number.isSafeInteger(planId) && planId > 0
          ? adminApi(`/admin/plans/${planId}/entitlements`)
          : Promise.reject(new Error("Invalid plan id")),
      ]);
      const failures: string[] = [];
      let nextRegistry: FeatureDefinition[] | null = null;
      let nextEntitlements: Record<string, PlanEntitlement> | null = null;

      if (registryResult.status === "fulfilled") {
        if (Array.isArray(registryResult.value?.data)) {
          nextRegistry = registryResult.value.data as FeatureDefinition[];
          setRegistry(nextRegistry);
        } else {
          failures.push("The feature registry response is malformed.");
        }
      } else {
        failures.push(registryResult.reason instanceof Error ? registryResult.reason.message : "Could not load the feature registry.");
      }

      if (planResult.status === "fulfilled") {
        if (planResult.value?.data?.entitlements && typeof planResult.value.data.entitlements === "object") {
          nextEntitlements = planResult.value.data.entitlements as Record<string, PlanEntitlement>;
        } else {
          failures.push("The plan entitlement response is malformed.");
        }
      } else {
        failures.push(planResult.reason instanceof Error ? planResult.reason.message : "Could not load plan entitlements.");
      }

      if (failures.length > 0 || nextRegistry === null || nextEntitlements === null) {
        throw new Error(failures.join(" ") || "Could not load plan entitlements.");
      }
      const loadedValues=valuesFromEntitlements(nextRegistry, nextEntitlements);
      setValues(loadedValues);
      setLimitedValues(Object.fromEntries(nextRegistry.filter(f=>f.type==="numeric").map(f=>[f.code,String(loadedValues[f.code]==="-1"?1:loadedValues[f.code]??0)])));
      setReady(true);
      return true;
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not load plan entitlements.");
      return false;
    } finally {
      setLoading(false);
    }
  }, [planId]);

  useEffect(() => { void reload(); }, [reload]);

  const groups = useMemo(() => groupFeatures(registry), [registry]);

  async function save(event: React.FormEvent) {
    event.preventDefault();
    setNotice("");
    const validated = validateEntitlementValues(registry, values);
    if (!validated.entitlements) { setError(validated.error || "Check entitlement values."); return; }
    setSaving(true);
    setError("");
    try {
      const saved = await saveAndReloadEntitlements(planId, validated.entitlements, adminApi);
      if (registry.some(feature => saved[feature.code]?.enabled !== validated.entitlements?.[feature.code]?.enabled || saved[feature.code]?.limit !== validated.entitlements?.[feature.code]?.limit)) {
        throw new Error("The saved entitlement values did not match the submitted values. Reload the plan and try again.");
      }
      setValues(valuesFromEntitlements(registry, saved));
      onSaved?.(saved, registry);
      setNotice("Plan entitlements saved successfully.");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not save plan entitlements.");
    } finally {
      setSaving(false);
    }
  }

  return <div className="fixed inset-0 z-[60] grid place-items-center bg-slate-900/40 p-4">
    <form onSubmit={save} className="max-h-[92vh] w-full max-w-3xl overflow-auto rounded-2xl bg-white p-6 shadow-xl">
      <div className="flex items-start justify-between gap-4"><div><h2 className="text-xl font-bold">Plan entitlements</h2><p className="mt-1 text-sm text-slate-500">{planName} · configure features available to subscribers.</p></div><button type="button" onClick={onClose} className="rounded-lg border px-3 py-1.5">Close</button></div>
      {notice && <p role="status" className="mt-4 rounded-xl bg-green-50 p-3 text-sm text-green-800">{notice}</p>}
      {error && <p role="alert" className="mt-4 rounded-xl bg-red-50 p-3 text-sm text-red-800">{error}<button type="button" onClick={() => void reload()} className="ml-3 font-semibold underline">Retry</button></p>}
      {loading ? <p role="status" className="mt-6 rounded-xl border p-8 text-center text-slate-500">Loading plan entitlements...</p> : !ready ? <p role="status" className="mt-6 rounded-xl border p-8 text-center text-slate-500">Entitlement values are unavailable until the plan loads successfully.</p> : registry.length === 0 ? <p className="mt-6 rounded-xl border p-8 text-center text-slate-500">No configurable features are available.</p> : <div className="mt-5 space-y-5">{groups.map(([group, features]) => <section key={group} aria-label={group}><h3 className="mb-2 font-semibold">{group}</h3><div className="grid gap-3 sm:grid-cols-2">{features.map(feature => <div key={feature.code} className="rounded-xl border p-4"><div className="flex items-start justify-between gap-3"><div><label htmlFor={`entitlement-${feature.code}`} className="font-medium">{feature.name}</label><p className="mt-1 text-xs leading-5 text-slate-500">{feature.description}</p></div>{feature.type === "boolean" ? <input id={`entitlement-${feature.code}`} type="checkbox" aria-label={feature.name} checked={values[feature.code] === true} onChange={e => setValues(current => ({ ...current, [feature.code]: e.target.checked }))} className="mt-1 h-5 w-5" /> : <div className="space-y-2"><NumericStepper label={`${feature.name} limit`} id={`entitlement-${feature.code}`} value={String(values[feature.code] === "-1" ? limitedValues[feature.code] ?? "1" : values[feature.code] ?? "0")} disabled={values[feature.code] === "-1"} onChange={value=>{setLimitedValues(current=>({...current,[feature.code]:value}));setValues(current=>({...current,[feature.code]:value}))}}/><label className="flex min-h-8 items-center gap-2 text-sm"><input type="checkbox" checked={values[feature.code] === "-1"} onChange={e=>setValues(current=>({...current,[feature.code]:e.target.checked?"-1":limitedValues[feature.code]??"1"}))}/>Unlimited</label></div>}</div></div>)}</div></section>)}</div>}
      <div className="mt-6 flex justify-end gap-3"><button type="button" onClick={onClose} className="rounded-xl border px-4 py-2">Cancel</button><button type="submit" disabled={loading || saving || !ready || registry.length === 0} className="rounded-xl bg-blue-600 px-4 py-2 font-semibold text-white disabled:opacity-50">{saving ? "Saving..." : "Save entitlements"}</button></div>
    </form>
  </div>;
}
