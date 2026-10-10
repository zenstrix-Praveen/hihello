const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const ts = require("typescript");

const source = fs.readFileSync(path.join(__dirname, "../src/lib/plan-entitlements.ts"), "utf8");
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText;
const moduleRef = { exports: {} };
new Function("module", "exports", compiled)(moduleRef, moduleRef.exports);
const { valuesFromEntitlements, validateEntitlementValues, groupFeatures, mergeEntitlementStorage, saveAndReloadEntitlements } = moduleRef.exports;

const registry = [
  { code: "digital_cards", name: "Digital cards", description: "", type: "numeric", group: "Cards" },
  { code: "analytics", name: "Analytics", description: "", type: "boolean", group: "Insights" },
  { code: "team_members", name: "Team members", description: "", type: "numeric", group: "Account" },
];

test("registry metadata groups dynamically", () => {
  assert.deepEqual(groupFeatures(registry).map(([name]) => name), ["Cards", "Insights", "Account"]);
  assert.equal(groupFeatures(registry)[0][1][0].name, "Digital cards");
});

test("saved entitlement values populate controls including booleans and unlimited", () => {
  assert.deepEqual(valuesFromEntitlements(registry, {
    digital_cards: { enabled: true, limit: 5 },
    analytics: { enabled: true, limit: 0 },
    team_members: { enabled: true, limit: -1 },
  }), { digital_cards: "5", analytics: true, team_members: "-1" });
});

test("successful save updates in-memory plan CRUD values without losing legacy fields", () => {
  const merged = mergeEntitlementStorage(["analytics", "Legacy marketing copy"], { old_limit: 12 }, {
    digital_cards: { enabled: true, limit: 10 },
    analytics: { enabled: false, limit: 0 },
    team_members: { enabled: true, limit: -1 },
  }, registry);
  assert.deepEqual(merged.features, ["Legacy marketing copy"]);
  assert.deepEqual(merged.limits, { old_limit: 12, digital_cards_limit: 10, team_members_limit: -1 });
});

test("boolean toggles and numeric values normalize to the CORE-002 contract", () => {
  for (const [raw, expectedEnabled] of [["0", false], ["5", true], ["-1", true]]) {
    const result = validateEntitlementValues(registry, { digital_cards: raw, analytics: true, team_members: "0" });
    assert.equal(result.entitlements.digital_cards.limit, Number(raw));
    assert.equal(result.entitlements.digital_cards.enabled, expectedEnabled);
    assert.equal(result.entitlements.analytics.enabled, true);
    assert.equal(result.entitlements.team_members.limit, 0);
  }
  const disabled = validateEntitlementValues(registry, { digital_cards: "5", analytics: false, team_members: "-1" });
  assert.equal(disabled.entitlements.analytics.enabled, false);
  assert.equal(disabled.entitlements.team_members.limit, -1);
});

test("limits below -1, decimal, and nonnumeric values are rejected", () => {
  for (const invalid of ["-2", "1.5", "abc", ""]) {
    assert.match(validateEntitlementValues(registry, { digital_cards: invalid, analytics: false, team_members: "0" }).error, /whole number/);
  }
});

test("save sends the normalized payload and reloads persisted values", async () => {
  const calls = [];
  const saved = { digital_cards: { enabled: true, limit: 10 }, analytics: { enabled: true, limit: 0 }, team_members: { enabled: true, limit: -1 } };
  const request = async (url, options) => {
    calls.push([url, options]);
    return options?.method === "PUT" ? { data: { plan_id: 9 } } : { data: { plan_id: 9, entitlements: saved } };
  };
  const payload = validateEntitlementValues(registry, { digital_cards: "10", analytics: true, team_members: "-1" }).entitlements;
  const result = await saveAndReloadEntitlements(9, payload, request);
  assert.equal(calls.length, 2);
  assert.equal(calls[0][1].method, "PUT");
  assert.deepEqual(JSON.parse(calls[0][1].body), { entitlements: payload });
  assert.deepEqual(result, saved);
});

test("save errors are surfaced and UI does not branch on plan names", async () => {
  await assert.rejects(saveAndReloadEntitlements(9, {}, async () => { throw new Error("Plan save failed"); }), /Plan save failed/);
  const component = fs.readFileSync(path.join(__dirname, "../src/components/admin/PlanEntitlementsEditor.tsx"), "utf8");
  assert.doesNotMatch(component, /plan(Name|\.code)\s*===/i);
  assert.match(component, /useEffect\(\(\) => \{ void reload\(\); \}, \[reload\]\)/);
  assert.match(component, /useCallback\(async \(\): Promise<boolean> =>/);
});
