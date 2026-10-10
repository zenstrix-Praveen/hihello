const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const ts = require('typescript');

function fixture() {
  const scripts = [];
  const timers = new Map();
  let timerID = 0;
  const context = {
    exports: {}, window: {},
    document: {
      createElement: () => ({ remove() { scripts.splice(scripts.indexOf(this), 1); } }),
      body: { appendChild: script => scripts.push(script) },
    },
    setTimeout: fn => { const id = ++timerID; timers.set(id, fn); return id; },
    clearTimeout: id => timers.delete(id),
  };
  const source = fs.readFileSync(require.resolve('../src/lib/razorpay-checkout.ts'), 'utf8');
  vm.runInNewContext(ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText, context);
  return { context, scripts, timers, load: context.exports.loadRazorpayCheckout };
}

test('concurrent checkout requests share one script, and loaded checkout is reused', async () => {
  const f = fixture();
  const first = f.load();
  const second = f.load();
  assert.equal(first, second);
  assert.equal(f.scripts.length, 1);
  assert.equal(f.scripts[0].src, 'https://checkout.razorpay.com/v1/checkout.js');
  f.context.window.Razorpay = function() {};
  f.scripts[0].onload();
  await first;
  await f.load();
  assert.equal(f.scripts.length, 1);
  assert.equal(f.timers.size, 0);
});

test('script failure is visible and a subsequent attempt can load again', async () => {
  const f = fixture();
  const first = f.load();
  f.scripts[0].onerror();
  await assert.rejects(first, /Unable to load secure checkout/);
  assert.equal(f.scripts.length, 0);
  const retry = f.load();
  assert.equal(f.scripts.length, 1);
  f.context.window.Razorpay = function() {};
  f.scripts[0].onload();
  await retry;
});

test('a script that never responds times out without leaving a duplicate', async () => {
  const f = fixture();
  const result = f.load();
  [...f.timers.values()][0]();
  await assert.rejects(result, /check your connection/);
  assert.equal(f.scripts.length, 0);
});
