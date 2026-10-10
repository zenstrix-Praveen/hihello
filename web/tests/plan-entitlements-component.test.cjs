const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const ts = require("typescript");

function loadComponent(dependencies = {}) {
  const file = path.join(__dirname, "../src/components/admin/PlanEntitlementsEditor.tsx");
  const code = ts.transpileModule(fs.readFileSync(file, "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2020 },
  }).outputText;
  const moduleRef = { exports: {} };
  const stepperFile = path.join(__dirname, "../src/components/admin/NumericStepper.tsx");
  const stepperCode = ts.transpileModule(fs.readFileSync(stepperFile, "utf8"), { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, target: ts.ScriptTarget.ES2020 } }).outputText;
  const stepperModule = { exports: {} };
  new Function("require", "module", "exports", stepperCode)(name => name === "react/jsx-runtime" ? require(name) : require(name), stepperModule, stepperModule.exports);
  const stepper = stepperModule.exports;
  new Function("require", "module", "exports", code)(name => dependencies[name] ?? (name === "@/components/admin/NumericStepper" ? { __esModule: true, default: stepper.default } : require(name)), moduleRef, moduleRef.exports);
  return moduleRef.exports.PlanEntitlementsEditor;
}

function loadEntitlementHelpers() {
  const file = path.join(__dirname, "../src/lib/plan-entitlements.ts");
  const code = ts.transpileModule(fs.readFileSync(file, "utf8"), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText;
  const moduleRef = { exports: {} };
  new Function("module", "exports", code)(moduleRef, moduleRef.exports);
  return moduleRef.exports;
}

function harness(component) {
  const states = [];
  const dependencies = [];
  const effects = [];
  let index = 0;
  const React = {
    useState(initial) {
      const current = index++;
      if (!(current in states)) states[current] = initial;
      return [states[current], value => { states[current] = typeof value === "function" ? value(states[current]) : value; }];
    },
    useCallback(callback) { index++; return callback; },
    useMemo(callback) { index++; return callback(); },
    useEffect(callback, deps) {
      const current = index++;
      if (!dependencies[current] || deps.some((value, position) => value !== dependencies[current][position])) {
        dependencies[current] = deps;
        effects.push(callback);
      }
    },
  };
  const Editor = loadComponent({
    react: React,
    "@/lib/admin-api": { adminApi: (...args) => harness.api(...args) },
    "@/lib/plan-entitlements": loadEntitlementHelpers(),
  });
  return { render: props => { index = 0; return Editor(props); }, effects };
}

harness.api = async () => { throw new Error("Harness API was not configured"); };

function find(node, predicate) {
  if (!node || typeof node !== "object") return undefined;
  if (predicate(node)) return node;
  if (typeof node.type === "function") return find(node.type(node.props), predicate);
  for (const child of [node.props?.children].flat(Infinity)) {
    const found = find(child, predicate);
    if (found) return found;
  }
}

function text(node) {
  if (node == null) return "";
  if (typeof node !== "object") return String(node);
  return [node.props?.children].flat(Infinity).map(text).join("");
}
const flush = () => new Promise(resolve => setImmediate(resolve));

const registry = [
  { code: "digital_cards", name: "Digital Cards", description: "Cards", type: "numeric", group: "Cards" },
  { code: "analytics", name: "Analytics", description: "Analytics", type: "boolean", group: "Features" },
];

test("valid plan ID loads registry and saved values, saves, and reloads persisted values", async () => {
  const stored = {
    digital_cards: { enabled: true, limit: 5 },
    analytics: { enabled: true, limit: 0 },
  };
  const calls = [];
  harness.api = async (url, options = {}) => {
    calls.push([url, options.method || "GET"]);
    if (url === "/admin/plans/feature-registry") return { data: registry };
    if (url !== "/admin/plans/42/entitlements") throw new Error(`Unexpected entitlement URL: ${url}`);
    if (options.method === "PUT") Object.assign(stored, JSON.parse(options.body).entitlements);
    return { data: { plan_id: 42, entitlements: structuredClone(stored) } };
  };
  const props = { planId: 42, planName: "OCTOBER", onClose() {} };
  let view = harness();
  view.render(props);
  await view.effects[0]();
  await flush();
  let tree = view.render(props);
  assert.deepEqual(calls.slice(0, 2), [["/admin/plans/feature-registry", "GET"], ["/admin/plans/42/entitlements", "GET"]]);
  assert.equal(find(tree, n => n.props?.["aria-label"] === "Digital Cards limit").props.value, "5");
  assert.equal(find(tree, n => n.props?.["aria-label"] === "Analytics").props.checked, true);

  find(tree, n => n.props?.["aria-label"] === "Digital Cards limit").props.onChange({ target: { value: "10" } });
  tree = view.render(props);
  await find(tree, n => n.type === "form").props.onSubmit({ preventDefault() {} });
  tree = view.render(props);
  assert.deepEqual(calls.slice(-2), [["/admin/plans/42/entitlements", "PUT"], ["/admin/plans/42/entitlements", "GET"]]);
  assert.equal(find(tree, n => n.props?.["aria-label"] === "Digital Cards limit").props.value, "10");

  view = harness();
  view.render(props);
  await view.effects[0]();
  await flush();
  tree = view.render(props);
  assert.equal(find(tree, n => n.props?.["aria-label"] === "Digital Cards limit").props.value, "10");
});

test("invalid plan ID shows the API error without claiming the registry is empty", async () => {
  const calls = [];
  harness.api = async url => {
    calls.push(url);
    if (url === "/admin/plans/feature-registry") return { data: registry };
    throw new Error("Invalid plan id");
  };
  const view = harness();
  const props = { planId: Number.NaN, planName: "OCTOBER", onClose() {} };
  view.render(props);
  await view.effects[0]();
  await flush();
  const tree = view.render(props);
  assert.deepEqual(calls, ["/admin/plans/feature-registry"]);
  assert.match(text(find(tree, n => n.props?.role === "alert")), /Invalid plan id/);
  assert.doesNotMatch(text(tree), /No configurable features are available/);
  assert.equal(find(tree, n => n.type === "button" && text(n).includes("Save entitlements")).props.disabled, true);
});
