const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
function load() {
  const file = path.join(__dirname, '../src/lib/locale-core.ts');
  if (!fs.existsSync(file)) return {};
  const mod = { exports: {} };
  new Function('module', 'exports', ts.transpileModule(fs.readFileSync(file, 'utf8'), {compilerOptions:{module:ts.ModuleKind.CommonJS}}).outputText)(mod, mod.exports);
  return mod.exports;
}
test('public, auth and customer preference survives reload on one frontend key without overwriting admin preference', () => {
  const {readLanguage, persistLanguage} = load();
  assert.equal(typeof readLanguage, 'function', 'locale persistence must exist');
  const values = new Map([['admin_language', 'ar']]);
  const storage = {getItem:k=>values.get(k), setItem:(k,v)=>values.set(k,v)};
  assert.equal(readLanguage(storage, 'frontend_language', ['en','ar']), 'en');
  persistLanguage(storage, 'frontend_language', 'ar');
  assert.equal(readLanguage(storage, 'frontend_language', ['en','ar']), 'ar');
  persistLanguage(storage, 'frontend_language', 'en');
  assert.equal(values.get('admin_language'), 'ar');
  values.set('frontend_language','fr');
  assert.equal(readLanguage(storage, 'frontend_language', ['en','ar']), 'en');
  assert.equal(readLanguage({getItem(){throw Error('denied')}}, 'frontend_language', ['en','ar']), 'en');
});
test('customer locale boundary is shared by public, auth and workspace routes and sets RTL for Arabic',()=>{
  const fs=require('node:fs');const path=require('node:path');
  const source=fs.readFileSync(path.join(__dirname,'../src/lib/customer-locale.tsx'),'utf8');
  assert.match(source,/storageKey="frontend_language"/);
  assert.match(source,/dir=\{language\s*===\s*'ar'\s*\?\s*'rtl'\s*:\s*'ltr'\}/);
  assert.match(source,/lang=\{language\}/);
  assert.match(source,/pathname\.startsWith\('\/admin'\)/);
});
test('translation preserves interpolation values and falls back to English', () => {
  const {translate} = load();
  assert.equal(typeof translate, 'function', 'shared dictionary lookup must exist');
  const catalogs = {ar:{'Welcome {name}':'مرحبًا {name}'},en:{}};
  assert.equal(translate(catalogs, 'ar', 'Welcome {name}', {name:'Alex $& {name}'}), 'مرحبًا Alex $& {name}');
  assert.equal(translate(catalogs, 'en', 'Welcome {name}', {name:'Alex'}), 'Welcome Alex');
  assert.equal(translate(catalogs, 'ar', 'QR'), 'QR');
});
