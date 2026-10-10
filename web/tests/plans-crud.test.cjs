const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");

const page = fs.readFileSync(path.join(__dirname, "../src/app/admin/plans/page.tsx"), "utf8");

test("plan create and edit send persisted fields and reload the list", () => {
  assert.match(page, /planEdit\?`\/admin\/plans\/\$\{planEdit\}`:"\/admin\/plans"/);
  assert.match(page, /method:planEdit\?"PUT":"POST"/);
  assert.match(page, /price:Number\(planForm\.price\)/);
  assert.match(page, /trial_days:Number\(planForm\.trial_days\)/);
  assert.match(page, /sort_order:Number\(planForm\.sort_order\)/);
  assert.match(page, /setPlanForm\(null\).*await load\(\)/);
  assert.match(page, /function editPlan\(p:Plan\).*const limits(?::Record<string,unknown>)?=\{\.\.\.blankPlan\(\)\.limits,\.\.\.\(p\.limits\|\|\{\}\)\}/);
});

test("plan status and archive mutations prevent duplicates and refresh persisted state", () => {
  assert.match(page, /async function togglePlan\(p:Plan\).*method:"PATCH".*await load\(\).*finally\{planActionLock\.current=false;setPlanActionId\(null\)\}/);
  assert.match(page, /async function removePlan\(p:Plan\).*method:"DELETE".*await load\(\).*finally\{planActionLock\.current=false;setPlanActionId\(null\)\}/);
  assert.equal((page.match(/disabled=\{planActionId!==null\}/g) || []).length, 2);
  assert.match(page, /catch\(e\)\{setError\(e instanceof Error\?e\.message:/);
});

test("plan form exposes billing, feature, limit, ordering, and active fields", () => {
  for (const field of ["price", "currency", "billing_period", "sort_order", "is_featured", "is_active", "limits"]) {
    assert.ok(page.includes(field), `expected Plans UI to contain ${field}`);
  }
  assert.match(page, /onClick=\{\(\)=>setEntitlementPlan\(p\)\}/);
  assert.match(page, /planId=\{entitlementPlan\.id\}/);
});

test("Add and Edit Plan render all five ordered sections in a scrollable modal with a persistent action bar", () => {
  const start = page.indexOf("{planForm&&");
  const end = page.indexOf("{entitlementPlan&&", start);
  const modal = page.slice(start, end);
  const sections = ["1. Basic Details", "2. Free Trial Configuration", "3. Usage Limits", "4. Included Features", "5. Plan Settings"];
  const config = fs.readFileSync(path.join(__dirname, "../src/components/admin/PlanConfigurationSections.tsx"), "utf8");
  assert.ok(modal.indexOf(sections[0]) < modal.indexOf(sections[1]));
  assert.ok(modal.indexOf(sections[1]) < modal.indexOf("<PlanConfigurationSections"));
  assert.ok(config.indexOf(sections[2]) < config.indexOf(sections[3]));
  assert.ok(modal.indexOf("<PlanConfigurationSections") < modal.indexOf(sections[4]));
  assert.match(modal, /max-h-\[calc\(100dvh-1\.5rem\)\].*overflow-hidden/);
  assert.match(modal, /flex-1 overflow-y-auto/);
  assert.match(modal, /shrink-0 justify-end.*Cancel/);
  assert.match(modal, /<PlanConfigurationSections limits=\{planForm\.limits\}/);
});

test("plan form provides the requested seven initial usage allowances and unlimited states", () => {
  const source = fs.readFileSync(path.join(__dirname, "../src/components/admin/PlanConfigurationSections.tsx"), "utf8");
  for (const [code, label, initial] of [["digital_cards_limit", "Digital Cards", 3], ["nfc_tags_limit", "NFC Tags", 2], ["qr_codes_limit", "QR Codes", 10], ["contacts_limit", "Contacts", 100], ["storage_limit", "Storage (GB)", 5], ["team_members_limit", "Team Members", 2], ["card_templates_limit", "Card Templates", 5]]) {
    assert.ok(source.includes(`"${code}", "${label}", ${initial}`));
  }
  assert.match(source, /disabled=\{unlimited\}/);
  assert.match(source, /_unlimited/);
  assert.match(source, /min=\{0\}/);
});
