const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');

const root = path.join(__dirname, '..');
const read = file => fs.readFileSync(path.join(root, file), 'utf8');

test('public mobile navigation exposes its menu, submenu, language selector and closes on navigation', () => {
  const header = read('src/components/public/PublicHeader.tsx');
  assert.match(header, /aria-controls="public-mobile-navigation"/);
  assert.match(header, /id="public-mobile-navigation"/);
  assert.match(header, /aria-expanded=\{mobileOpen\}/);
  assert.match(header, /mobileOpen\s*\?\s*"max-h-\[calc\(100dvh-72px\)\] opacity-100"/);
  assert.match(header, /onClick=\{\(\) => \{ setMobileOpen\(false\); setMobilePlatformOpen\(false\); \}\}/);
  assert.match(header, /setMobilePlatformOpen\(value => !value\)/);
  assert.match(header, /<CustomerLanguageSwitcher \/>/);
  assert.match(header, /fallbackOnUnavailable: true/);
  assert.match(header, /min-\[1280px\]:hidden/);
});

test('Next development server explicitly allows the LAN host used by mobile browsers', () => {
  const config = read('next.config.ts');
  assert.match(config, /allowedDevOrigins:\s*\["192\.168\.29\.31"\]/);
});

test('customer workspace closes either overlay before opening the other', () => {
  const layout = read('src/app/dashboard/layout.tsx');
  const header = read('src/components/dashboard/UserHeader.tsx');
  const sidebar = read('src/components/dashboard/UserSidebar.tsx');
  assert.match(layout, /dbizcard:close-navigation/);
  assert.match(layout, /dbizcard:close-chat-request/);
  assert.match(layout, /dbizcard:close-chat/);
  assert.match(layout, /onMenu=\{\(\) => setOpen\(value => !value\)\}/);
  assert.match(header, /dbizcard:close-chat-request/);
  assert.match(sidebar, /fixed top-20 bottom-0 start-0/);
});

test('LAN development API requests follow the frontend host even with localhost in .env.local', () => {
  const source = read('src/lib/api-config.ts');
  const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS } }).outputText;
  const mod = { exports: {} };
  new Function('module', 'exports', output)(mod, mod.exports);
  const oldWindow = global.window;
  const oldNodeEnv = process.env.NODE_ENV;
  const oldApi = process.env.NEXT_PUBLIC_API_URL;
  try {
    process.env.NODE_ENV = 'development';
    process.env.NEXT_PUBLIC_API_URL = 'http://localhost:8080/api/v1';
    global.window = { location: { hostname: '192.168.29.31', protocol: 'http:' } };
    assert.equal(mod.exports.getApiBaseUrl(), 'http://192.168.29.31:8080/api/v1');
    process.env.NEXT_PUBLIC_API_URL = 'https://api.example.com/api/v1';
    assert.equal(mod.exports.getApiBaseUrl(), 'https://api.example.com/api/v1');
  } finally {
    if (oldWindow === undefined) delete global.window;
    else global.window = oldWindow;
    if (oldNodeEnv === undefined) delete process.env.NODE_ENV;
    else process.env.NODE_ENV = oldNodeEnv;
    if (oldApi === undefined) delete process.env.NEXT_PUBLIC_API_URL;
    else process.env.NEXT_PUBLIC_API_URL = oldApi;
  }
});

test('mobile overflow is contained while data tables and card-builder steps remain horizontally usable', () => {
  const css = read('src/app/globals.css');
  const billing = read('src/components/dashboard/Billing.tsx');
  const editor = read('src/components/dashboard/CardEditor.tsx');
  assert.match(css, /overflow-x:clip/);
  assert.match(billing, /overflow-x-auto[\s\S]*<table/);
  assert.match(editor, /overflow-x-auto[\s\S]*min-w-max/);
});
