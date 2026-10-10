const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const ts = require("typescript");

test("stale customer 401 cannot clear a freshly saved login token", async () => {
  const source = fs.readFileSync(path.join(__dirname, "../src/lib/user-api.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  }).outputText;
  const apiModule = { exports: {} };
  new Function("module", "exports", "require", compiled)(apiModule, apiModule.exports, id => id === "@/lib/api-config" ? { getApiBaseUrl: () => "http://localhost:8080/api/v1" } : require(id));

  const oldGlobals = {
    localStorage: global.localStorage,
    window: global.window,
    fetch: global.fetch,
  };
  const values = new Map([["user_token", "old.token.value"]]);
  const events = [];
  global.localStorage = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, String(value)),
    removeItem: (key) => values.delete(key),
  };
  global.window = { dispatchEvent: (event) => events.push(event.type) };

  try {
    let rejectOldRequest;
    const requests = [];
    global.fetch = (url, options) => {
      requests.push(options);
      if (requests.length === 1) {
        return new Promise((resolve) => { rejectOldRequest = resolve; });
      }
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ id: 42 }) });
    };

    const staleRequest = apiModule.exports.apiRequest("/user/me").catch(() => null);
    assert.equal(requests[0].headers.get("Authorization"), "Bearer old.token.value");

    apiModule.exports.saveUserToken("fresh.token.value");
    rejectOldRequest({
      ok: false,
      status: 401,
      json: async () => ({ error: "A valid user session is required" }),
    });
    await staleRequest;

    assert.equal(apiModule.exports.getUserToken(), "fresh.token.value");
    assert.deepEqual(events, []);

    await apiModule.exports.apiRequest("/user/me");
    assert.equal(requests[1].headers.get("Authorization"), "Bearer fresh.token.value");

    global.fetch = async () => ({
      ok: false,
      status: 401,
      json: async () => ({ error: "A valid user session is required" }),
    });
    await apiModule.exports.apiRequest("/user/me").catch(() => null);
    assert.equal(apiModule.exports.getUserToken(), null);
    assert.deepEqual(events, ["customer:unauthorized"]);
  } finally {
    for (const [key, value] of Object.entries(oldGlobals)) {
      if (value === undefined) delete global[key];
      else global[key] = value;
    }
  }
});
