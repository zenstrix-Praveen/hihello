const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');

function load(file, mocks = {}) {
  const output = ts.transpileModule(fs.readFileSync(path.join(__dirname, '../src', file), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020, jsx: ts.JsxEmit.ReactJSX },
  }).outputText;
  const mod = { exports: {} };
  new Function('module', 'exports', 'require', output)(mod, mod.exports, id => id in mocks ? mocks[id] : id === '@/lib/api-config' ? { getApiBaseUrl: () => 'http://localhost:8080/api/v1' } : id === '@/lib/customer-upgrade' ? load('lib/customer-upgrade.ts') : require(id));
  return mod.exports;
}
function setup(t) {
  const saved = {}; const cleanups = [];
  for (const key of ['window', 'localStorage', 'sessionStorage', 'fetch', 'setTimeout']) saved[key] = global[key];
  const storage = () => { const data = new Map(); return { getItem: k => data.get(k) ?? null, setItem: (k,v) => data.set(k,String(v)), removeItem: k => data.delete(k) }; };
  global.localStorage = storage(); global.sessionStorage = storage();
  const events = new EventTarget();
  global.window = { addEventListener: events.addEventListener.bind(events), removeEventListener: events.removeEventListener.bind(events), dispatchEvent: events.dispatchEvent.bind(events), setInterval: () => 0 };
  t.after(() => { for (const cleanup of cleanups) cleanup(); for (const [k,v] of Object.entries(saved)) { if (v === undefined) delete global[k]; else global[k] = v; } });
  const api = load('lib/user-api.ts'); api.cleanups = cleanups; return api;
}
function component(file, api, initial = {}, extras = {}) {
  const states = [], effects = [], redirects = []; let index = 0;
  const router = { replace: p => redirects.push(p), push: p => redirects.push(p), refresh() {} };
  const hooks = {
    useState: value => { const i = index++; if (!(i in states)) states[i] = i in initial ? initial[i] : value; return [states[i], v => { states[i] = typeof v === 'function' ? v(states[i]) : v; }]; },
    useEffect: fn => effects.push(fn), useRef: value => ({ current: value }),
  };
  const stub = () => null;
  const mocks = { react: hooks, 'next/navigation': { useRouter: () => router, usePathname: () => '/dashboard', useSearchParams: () => new URLSearchParams() }, '@/lib/user-api': api,
    '@/lib/customer-locale': { useCustomerLocale: () => ({ t: (s, p = {}) => s.replace(/\{(\w+)\}/g, (_, k) => p[k] ?? ''), error: s => s, language: 'en', ready: true }) },
    '@/components/locale/CustomerLanguageSwitcher': { default: stub },
    'next/link': stub, 'lucide-react': {}, '@/components/brand/BrandLogo': stub,
    '@/components/public/PublicHeader': stub, '@/components/public/PublicFooter': stub,
    '@/components/dashboard/UserSidebar': stub, '@/components/dashboard/UserHeader': stub,
    '@/components/dashboard/UserSession': { UserContext: { Provider: 'provider' } },
    '@/components/public/Chatbot': { ChatbotLauncher: stub }, ...extras };
  const Page = load(file, mocks).default;
  return { states, effects, redirects, render: () => { index = 0; const tree = Page({ children: 'protected-content' }); return file === 'app/register/page.tsx' ? tree.props.children.type() : tree; } };
}
function find(node, predicate) {
  if (!node || typeof node !== 'object') return;
  if (predicate(node)) return node;
  for (const child of [node.props?.children].flat(Infinity)) { const found = find(child, predicate); if (found) return found; }
}
const flush = () => new Promise(resolve => setImmediate(resolve));
const response = (data, status = 200) => ({ ok: status < 400, status, json: async () => data, text: async () => JSON.stringify(data) });

test('customer support understands Arabic plan and card questions with reusable translation parameters', () => {
  const { localKnowledgeProvider } = load('lib/chatbot-knowledge.ts');
  const plan = localKnowledgeProvider.answer('ما هي خطتي الحالية', 'customer', { planName: 'Premium' });
  assert.equal(plan.params.plan, 'Premium');
  assert(plan.translationKey.includes('{plan}'));
  const cards = localKnowledgeProvider.answer('كم عدد بطاقاتي', 'customer', { cardCount: 2, publishedCards: 1 });
  assert.equal(cards.params.count, 2);
  assert(cards.answer.includes('2 cards'));
  assert(localKnowledgeProvider.answer('كيف أنشئ بطاقة رقمية', 'customer'));
});

test('customer login renders Arabic labels and validates required fields without a request', async t => {
  const api = setup(t);
  let requests = 0;
  global.fetch = async () => { requests++; return response({}); };
  const messages = { 'Customer Access': 'وصول العميل', 'Email is required.': 'البريد الإلكتروني مطلوب.' };
  const view = component('app/login/customer/page.tsx', api, {}, {
    '@/lib/customer-locale': { useCustomerLocale: () => ({ t: s => messages[s] || s, error: s => messages[s] || s, language: 'ar', ready: true }) },
  });
  assert(find(view.render(), n => n.props?.children === 'وصول العميل'));
  await find(view.render(), n => n.type === 'form').props.onSubmit({ preventDefault() {} });
  assert.equal(requests, 0);
  assert(find(view.render(), n => n.props?.children === 'البريد الإلكتروني مطلوب.'));
});

test('login saves customer credentials and opens dashboard without changing admin credentials', async t => {
  const api = setup(t); localStorage.setItem('admin_token', 'admin');
  global.fetch = async (url, options) => { assert.equal(JSON.parse(options.body).email, 'customer@example.com'); return response({ token: 'customer' }); };
  const view = component('app/login/customer/page.tsx', api, { 0: 'customer@example.com', 1: 'password' });
  await find(view.render(), n => n.type === 'form').props.onSubmit({ preventDefault() {} });
  assert.equal(localStorage.getItem('user_token'), 'customer'); assert.equal(localStorage.getItem('admin_token'), 'admin');
  assert.deepEqual(view.redirects, ['/dashboard']);
});

test('dashboard waits for persisted session validation before rendering protected content', async t => {
  const api = setup(t); api.saveUserToken('valid'); let finish;
  global.fetch = () => new Promise(resolve => { finish = resolve; });
  const view = component('app/dashboard/layout.tsx', api);
  assert(find(view.render(), n => n.props?.role === 'status'));
  const cleanup = view.effects[0](); api.cleanups.push(cleanup);
  assert.deepEqual(view.redirects, []);
  finish(response({ id: 42, name: 'Customer' })); await flush();
  assert.equal(view.render().type, 'provider'); assert.deepEqual(view.redirects, []);
});

test('dashboard ignores a successful validation from a logged-out session', async t => {
  const api = setup(t); api.saveUserToken('old'); let finish;
  global.fetch = () => new Promise(resolve => { finish = resolve; });
  const view = component('app/dashboard/layout.tsx', api); view.render(); const cleanup = view.effects[0](); api.cleanups.push(cleanup);
  api.logoutUser(); finish(response({ id: 42 })); await flush();
  assert.equal(view.states[0], null); assert(view.redirects.includes('/login'));
});

test('dashboard ignores stale errors after a new session is saved', async t => {
  const api = setup(t); api.saveUserToken('old'); let finish;
  global.fetch = () => new Promise(resolve => { finish = resolve; });
  const view = component('app/dashboard/layout.tsx', api); view.render(); const cleanup = view.effects[0](); api.cleanups.push(cleanup);
  api.saveUserToken('new'); finish(response({ error: 'A valid user session is required' }, 401)); await flush();
  assert.equal(view.states[1], ''); assert.equal(api.getUserToken(), 'new'); assert.deepEqual(view.redirects, []);
});

for (const status of ['missing', 'invalid', 'expired']) test(`${status} session redirects cleanly and preserves admin`, async t => {
  const api = setup(t); localStorage.setItem('admin_token','admin'); if (status !== 'missing') api.saveUserToken(status);
  global.fetch = async () => response({ error: 'A valid user session is required' }, 401);
  const view = component('app/dashboard/layout.tsx', api); view.render(); const cleanup = view.effects[0](); api.cleanups.push(cleanup); await flush();
  assert.equal(api.getUserToken(), null); assert.equal(localStorage.getItem('admin_token'), 'admin'); assert.deepEqual(view.redirects, ['/login']);
});

test('successful reset clears the old customer session and preserves admin', async t => {
  const api = setup(t); api.saveUserToken('old'); localStorage.setItem('admin_token', 'admin'); sessionStorage.setItem('auth_reset_token', 'reset');
  global.setTimeout = fn => { fn(); return 0; };
  global.fetch = async (url, options) => { assert.deepEqual(JSON.parse(options.body), { email:'customer@example.com', reset_token:'reset', new_password:'new-password' }); return response({ message:'Password reset successfully' }); };
  const view = component('app/reset-password/page.tsx', api, { 0:'customer@example.com', 1:'reset', 2:'new-password', 3:'new-password' });
  await find(view.render(), n => n.type === 'form').props.onSubmit({ preventDefault() {} });
  assert.equal(api.getUserToken(), null); assert.equal(localStorage.getItem('admin_token'), 'admin'); assert.equal(sessionStorage.getItem('auth_reset_token'), null); assert.deepEqual(view.redirects, ['/login']);
});

test('admin 401 clears only admin credentials', async t => {
  const api = setup(t); api.saveUserToken('customer'); localStorage.setItem('admin_token','admin');
  global.fetch = async () => response({ error:'Expired' },401);
  await assert.rejects(load('lib/admin-api.ts').adminApi('/admin/me'));
  assert.equal(api.getUserToken(),'customer'); assert.equal(localStorage.getItem('admin_token'),null);
});

test('registration sends validated values and proceeds to OTP without authenticating', async t => {
  const api = setup(t);
  global.fetch = async (url, options) => { assert(url.endsWith('/auth/register')); assert.equal(JSON.parse(options.body).password, 'password123'); return response({ message:'Verify email' },201); };
  const view = component('app/register/page.tsx', api, { 0:'Customer', 1:'Customer@example.com', 2:'', 3:'password123', 4:'password123' });
  await find(view.render(), n => n.type === 'form').props.onSubmit({ preventDefault() {} });
  assert.equal(sessionStorage.getItem('auth_email'),'customer@example.com'); assert.equal(api.getUserToken(),null);
  assert.deepEqual(view.redirects,['/verify-otp?purpose=customer_registration']);
});

for (const valid of [true,false]) test(`registration OTP ${valid ? 'success saves session' : 'failure does not authenticate'}`, async t => {
  const api = setup(t);
  global.fetch = async (url, options) => { assert(url.endsWith('/auth/verify-otp')); assert.equal(JSON.parse(options.body).purpose,'customer_registration'); return valid ? response({token:'verified'}) : response({error:'The code is invalid or expired'},400); };
  const view = component('components/auth/CustomerOTPForm.tsx',api,{0:'customer@example.com',1:['1','2','3','4','5','6']});
  await find(view.render(), n => n.type === 'form').props.onSubmit({preventDefault(){}});
  assert.equal(api.getUserToken(),valid ? 'verified' : null); assert.deepEqual(view.redirects,valid ? ['/dashboard'] : []);
  if (!valid) assert.equal(view.states[3],'The code is invalid or expired');
});

test('forgot password initiates customer verification without touching admin reset state', async t => {
  const api = setup(t); sessionStorage.setItem('admin_reset_token','admin-reset');
  global.fetch = async (url, options) => { assert(url.endsWith('/auth/forgot-password')); assert.equal(JSON.parse(options.body).email,'customer@example.com'); return response({message:'If an account exists, a verification code has been sent.'}); };
  const view = component('app/forgot-password/page.tsx',api,{0:'customer@example.com'});
  await find(view.render(),n => n.type === 'form').props.onSubmit({preventDefault(){}});
  assert.deepEqual(view.redirects,['/verify-otp?purpose=customer_password_reset']); assert.equal(sessionStorage.getItem('admin_reset_token'),'admin-reset');
});

test('customer sidebar logout is idempotent, opens login and preserves admin', t => {
  const api = setup(t); api.saveUserToken('customer'); localStorage.setItem('admin_token','admin');
  const view = component('components/dashboard/UserSidebar.tsx',api);
  const button = find(view.render(), n => n.type === 'button' && [n.props.children].flat().includes('Logout'));
  button.props.onClick(); button.props.onClick();
  assert.equal(api.getUserToken(),null); assert.equal(localStorage.getItem('admin_token'),'admin'); assert.deepEqual(view.redirects,['/login','/login']);
});

test('admin logout preserves customer session', t => {
  const api = setup(t); api.saveUserToken('customer'); localStorage.setItem('admin_token','admin'); localStorage.setItem('admin_user','{}');
  const view = component('components/admin/AdminHeader.tsx',api,{}, {'@/lib/admin-api':{},'@/lib/admin-locale':{useAdminLocale:()=>({language:'en',languages:[],t:s=>s})}});
  find(view.render(), n => n.props?.['aria-label'] === 'Logout').props.onClick();
  assert.equal(api.getUserToken(),'customer'); assert.equal(localStorage.getItem('admin_token'),null); assert.equal(localStorage.getItem('admin_user'),null); assert.deepEqual(view.redirects,['/admin/login']);
});

test('password reset OTP stores reset verification without creating a login session', async t => {
  const api = setup(t);
  const router = { replace: p => redirects.push(p) }; const redirects = [];
  global.fetch = async (url, options) => { assert.equal(JSON.parse(options.body).purpose,'customer_password_reset'); return response({reset_token:'reset-verification'}); };
  const view = component('components/auth/CustomerOTPForm.tsx',api,{0:'customer@example.com',1:['1','2','3','4','5','6']}, {'next/navigation':{useRouter:()=>router,useSearchParams:()=>new URLSearchParams('purpose=customer_password_reset')}});
  await find(view.render(),n => n.type === 'form').props.onSubmit({preventDefault(){}});
  assert.equal(sessionStorage.getItem('auth_reset_token'),'reset-verification'); assert.equal(api.getUserToken(),null); assert.deepEqual(redirects,['/reset-password']);
});

test('invalid reset verification shows structured error and does not discard a valid session', async t => {
  const api = setup(t); api.saveUserToken('valid');
  global.fetch = async () => response({error:'Verification is invalid or expired'},400);
  const view = component('app/reset-password/page.tsx',api,{0:'customer@example.com',1:'invalid',2:'new-password',3:'new-password'});
  await find(view.render(),n => n.type === 'form').props.onSubmit({preventDefault(){}});
  assert.equal(view.states[6],'Verification is invalid or expired'); assert.equal(api.getUserToken(),'valid'); assert.deepEqual(view.redirects,[]);
});

