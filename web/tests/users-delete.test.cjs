const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const ts = require('typescript');

function view(api) {
  const slots = [], effects = []; let index = 0;
  const memo = (fn, deps) => {
    const i = index++;
    if (!slots[i] || deps.some((v, n) => v !== slots[i].deps[n])) slots[i] = { value: fn(), deps };
    return slots[i].value;
  };
  const react = {
    useState(value) { const i = index++; if (!(i in slots)) slots[i] = value; return [slots[i], v => slots[i] = typeof v === 'function' ? v(slots[i]) : v]; },
    useRef: value => memo(() => ({ current: value }), []),
    useCallback: (fn, deps) => memo(() => fn, deps),
    useEffect: (fn, deps) => memo(() => effects.push(fn), deps),
  };
  const code = ts.transpileModule(fs.readFileSync(`${__dirname}/../src/app/admin/users/page.tsx`, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: false } }).outputText;
  const mod = { exports: {} };
  const deps = { react, '@/lib/admin-api': { adminApi: api }, '@/lib/admin-location': { useAdminLocation: () => ({ search: '' }) } };
  new Function('require', 'module', 'exports', code)(n => deps[n] ?? require(n), mod, mod.exports);
  return { render() { index = 0; return mod.exports.default(); }, async settle() { for (let i = 0; i < 6; i++) { this.render(); for (const effect of effects.splice(0)) effect(); await new Promise(setImmediate); } } };
}
function text(n) { return n == null ? '' : typeof n !== 'object' ? String(n) : [n.props?.children].flat(Infinity).map(text).join(''); }
function find(n, p) { if (!n || typeof n !== 'object') return; if (p(n)) return n; for (const c of [n.props?.children].flat(Infinity)) { const found = find(c, p); if (found) return found; } }
const dialog = v => find(v.render(), n => n.props?.role === 'dialog');
const button = (v, label, root = v.render()) => find(root, n => n.type === 'button' && text(n) === label);
const org = id => ({ id, name: id === 1 ? 'ADM USER DELETE TEST' : `User ${id}`, status: 'active', created_at: '2026-01-01', user_count: 0, card_count: 0 });
function store(count = 1) {
  let rows = Array.from({ length: count }, (_, i) => org(i + 1)); const calls = [];
  return { calls, async api(path, options = {}) {
    calls.push([path, options.method || 'GET']);
    if (options.method === 'DELETE') { rows = rows.filter(o => o.id !== Number(path.split('/').at(-1))); return { success: true }; }
    const query = new URL(path, 'http://local').searchParams;
    const filtered = rows.filter(o => o.name.includes(query.get('search') || '') && (!query.get('status') || o.status === query.get('status')));
    const page = Number(query.get('page') || 1);
    return { data: filtered.slice((page - 1) * 20, page * 20), pagination: { page, limit: 20, total: filtered.length, pages: Math.ceil(filtered.length / 20) } };
  } };
}

test('delete confirms, cancels, persists, refreshes count and survives remount', async () => {
  const s = store(2); const v = view(s.api); await v.settle(); assert(button(v, 'View')); assert(button(v, 'Edit')); assert(button(v, 'Suspend'));
  button(v, 'Delete').props.onClick(); assert.match(text(dialog(v)), /ADM USER DELETE TEST/);
  button(v, 'Cancel', dialog(v)).props.onClick(); assert(!s.calls.some(c => c[1] === 'DELETE'));
  button(v, 'Delete').props.onClick(); await button(v, 'Delete', dialog(v)).props.onClick(); await v.settle();
  assert.match(text(v.render()), /User deleted successfully\./); assert.doesNotMatch(text(v.render()), /ADM USER DELETE TEST/);
  assert.match(text(v.render()), /1 users/); assert.equal(s.calls.filter(c => c[1] === 'DELETE').length, 1);
  const refreshed = view(s.api); await refreshed.settle(); assert.doesNotMatch(text(refreshed.render()), /ADM USER DELETE TEST/);
});

test('protected user error preserves row; rapid clicks submit once', async () => {
  const s = store(); let reject, mutations = 0;
  const v = view((p, o = {}) => o.method === 'DELETE' ? (mutations++, new Promise((_, r) => reject = r)) : s.api(p, o)); await v.settle();
  button(v, 'Delete').props.onClick(); const confirm = button(v, 'Delete', dialog(v)); const pending = confirm.props.onClick(); confirm.props.onClick();
  assert.equal(mutations, 1); assert(button(v, 'Deleting…', dialog(v)).props.disabled); assert(button(v, 'Cancel', dialog(v)).props.disabled);
  reject(Error('You cannot delete the user matching your currently authenticated Admin account.')); await pending;
  assert.match(text(v.render()), /currently authenticated Admin account/); assert.match(text(v.render()), /ADM USER DELETE TEST/);
  assert.doesNotMatch(text(v.render()), /User deleted successfully/);
});

test('last row on final page returns to a valid page with updated count', async () => {
  const s = store(21), v = view(s.api); await v.settle(); button(v, 'Next').props.onClick(); await v.settle();
  assert.match(text(v.render()), /Page 2 of 2/); button(v, 'Delete').props.onClick(); await button(v, 'Delete', dialog(v)).props.onClick(); await v.settle();
  assert.match(text(v.render()), /Page 1 of 1/); assert.match(text(v.render()), /20 users/); assert.doesNotMatch(text(v.render()), /User 21/);
});

test('deleting only search result preserves filters and shows an empty state', async () => {
  const s = store(2), v = view(s.api); await v.settle();
  find(v.render(), n => n.props?.['aria-label'] === 'Search users').props.onChange({ target: { value: 'ADM USER DELETE TEST' } });
  find(v.render(), n => n.props?.['aria-label'] === 'Filter status').props.onChange({ target: { value: 'active' } }); await v.settle();
  button(v, 'Delete').props.onClick(); await button(v, 'Delete', dialog(v)).props.onClick(); await v.settle();
  assert.match(text(v.render()), /No users found/);
  assert.equal(find(v.render(), n => n.props?.['aria-label'] === 'Search users').props.value, 'ADM USER DELETE TEST');
  assert.equal(find(v.render(), n => n.props?.['aria-label'] === 'Filter status').props.value, 'active');
  assert(s.calls.at(-1)[0].includes('status=active'));
});

test('failed delete shows safe failure and does not remove the row', async () => {
  const s = store(), v = view((p, o = {}) => o.method === 'DELETE' ? Promise.reject(Error('Unexpected internal exception')) : s.api(p, o)); await v.settle();
  button(v, 'Delete').props.onClick(); await button(v, 'Delete', dialog(v)).props.onClick();
  assert.match(text(v.render()), /Could not delete user\./); assert.doesNotMatch(text(v.render()), /internal exception/); assert.match(text(v.render()), /ADM USER DELETE TEST/);
});

test('failed refresh after successful deletion clears stale rows and offers retry', async () => {
  const s = store(); let deleted = false;
  const v = view(async (p, o = {}) => {
    if (deleted && !o.method) throw Error('Could not load users');
    const response = await s.api(p, o); if (o.method === 'DELETE') deleted = true; return response;
  }); await v.settle();
  button(v, 'Delete').props.onClick(); await button(v, 'Delete', dialog(v)).props.onClick();
  assert.doesNotMatch(text(v.render()), /ADM USER DELETE TEST/);
  assert.match(text(v.render()), /User deleted successfully/);
  assert.match(text(v.render()), /list could not be reloaded/); assert(button(v, 'Retry'));
});

test('an old search response cannot overwrite the current user list', async () => {
  const s = store(2); let resolve;
  const v = view((p, o) => p.includes('search=old') ? new Promise(r => resolve = r) : s.api(p, o)); await v.settle();
  const search = () => find(v.render(), n => n.props?.['aria-label'] === 'Search users');
  search().props.onChange({ target: { value: 'old' } }); await v.settle();
  search().props.onChange({ target: { value: 'User 2' } }); await v.settle();
  resolve({ data: [org(1)], pagination: { total: 1, pages: 1 } }); await v.settle();
  assert.match(text(v.render()), /User 2/); assert.doesNotMatch(text(v.render()), /ADM USER DELETE TEST/);
});
