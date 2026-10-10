export type FeatureDefinition = {
  code: string;
  name: string;
  description: string;
  type: "boolean" | "numeric";
  group: string;
};

export type PlanEntitlement = { enabled: boolean; limit: number };
export type EntitlementValues = Record<string, boolean | string>;

export function valuesFromEntitlements(
  registry: FeatureDefinition[],
  entitlements: Record<string, PlanEntitlement>,
): EntitlementValues {
  return Object.fromEntries(registry.map((feature) => {
    const saved = entitlements[feature.code];
    return [feature.code, feature.type === "boolean" ? Boolean(saved?.enabled) : String(saved?.limit ?? 0)];
  }));
}

export function groupFeatures(registry: FeatureDefinition[]): [string, FeatureDefinition[]][] {
  const grouped = new Map<string, FeatureDefinition[]>();
  for (const feature of registry) {
    const group = feature.group || "Features";
    grouped.set(group, [...(grouped.get(group) ?? []), feature]);
  }
  return [...grouped.entries()];
}

export function mergeEntitlementStorage(
  features: string[],
  limits: Record<string, unknown>,
  entitlements: Record<string, PlanEntitlement>,
  registry: FeatureDefinition[],
): { features: string[]; limits: Record<string, unknown> } {
  const booleanCodes = new Set(registry.filter((feature) => feature.type === "boolean").map((feature) => feature.code));
  const nextFeatures = features.filter((code) => !booleanCodes.has(code.trim().toLowerCase()));
  for (const feature of registry) {
    if (feature.type === "boolean" && entitlements[feature.code]?.enabled) nextFeatures.push(feature.code);
  }
  const nextLimits = { ...limits };
  for (const feature of registry) {
    if (feature.type === "numeric") nextLimits[`${feature.code}_limit`] = entitlements[feature.code]?.limit ?? 0;
  }
  return { features: nextFeatures, limits: nextLimits };
}

export async function saveAndReloadEntitlements(
  planId: number,
  entitlements: Record<string, PlanEntitlement>,
  request: (path: string, options?: RequestInit) => Promise<any>,
): Promise<Record<string, PlanEntitlement>> {
  await request(`/admin/plans/${planId}/entitlements`, {
    method: "PUT",
    body: JSON.stringify({ entitlements }),
  });
  const saved = await request(`/admin/plans/${planId}/entitlements`);
  if (!saved?.data?.entitlements || typeof saved.data.entitlements !== "object") {
    throw new Error("The saved entitlement response is malformed.");
  }
  return saved.data.entitlements as Record<string, PlanEntitlement>;
}

export function validateEntitlementValues(
  registry: FeatureDefinition[],
  values: EntitlementValues,
): { entitlements?: Record<string, PlanEntitlement>; error?: string } {
  const entitlements: Record<string, PlanEntitlement> = {};
  for (const feature of registry) {
    const value = values[feature.code];
    if (feature.type === "boolean") {
      entitlements[feature.code] = { enabled: value === true, limit: 0 };
      continue;
    }
    if (typeof value !== "string" || !/^-?\d+$/.test(value.trim())) {
      return { error: `${feature.name}: limit must be -1, 0, or a positive whole number.` };
    }
    const limit = Number(value);
    if (!Number.isSafeInteger(limit) || limit < -1) {
      return { error: `${feature.name}: limit must be -1, 0, or a positive whole number.` };
    }
    entitlements[feature.code] = { enabled: limit !== 0, limit };
  }
  return { entitlements };
}
