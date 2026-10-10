const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');

function load(file, deps = {}) {
  const name = path.join(__dirname, '../src', file);
  if (!fs.existsSync(name)) return {};
  const mod = { exports: {} };
  const code = ts.transpileModule(fs.readFileSync(name, 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020, jsx: ts.JsxEmit.ReactJSX } }).outputText;
  new Function('module', 'exports', 'require', code)(mod, mod.exports, id => id in deps ? deps[id] : id === '@/lib/customer-upgrade' ? load('lib/customer-upgrade.ts') : id==='@/lib/global-pricing'?load('lib/global-pricing.ts'):id==='@/components/pricing/CurrencySelection'?{default:()=>null,CurrencyDropdown:()=>null,usePricingCurrency:()=>({currency:'',update(){}}),useCurrencyChoice:()=>({choice:{currency:'',country:''},update(){}})}:require(id));
  return mod.exports;
}
function find(node, predicate) {
  if (!node || typeof node !== 'object') return;
  if (predicate(node)) return node;
  for (const child of [node.props?.children].flat(Infinity)) { const found = find(child, predicate); if (found) return found; }
}
const professional = { id: 10, name: 'Professional', description: '', price: 25, currency: 'QAR', billing_period: 'monthly', pricing_type: 'fixed', trial_mode: 'limited', trial_days: 20, cta_url: '/pricing' };
const business = { ...professional, id: 14, name: 'Business', currency: 'SAR', price: 100, cta_url: '' };
const free = { ...professional, id: 31, name: 'Normal Users', price: 0, pricing_type: 'free', billing_period: 'lifetime', cta_url: '' };
const pricing = load('lib/pricing-plans.ts');
const PlanCard = load('components/pricing/PlanPricingCard.tsx', { '@/lib/pricing-plans': pricing }).default;
function setup(t, location = {}) {
  const previous = { window: global.window, localStorage: global.localStorage };
  const storage = new Map();
  global.localStorage = { getItem: k => storage.get(k) ?? null, setItem: (k,v) => storage.set(k,v), removeItem: k => storage.delete(k) };
  const events = new EventTarget();
  global.window = { location: { pathname: '/pricing', search: '', ...location }, dispatchEvent: events.dispatchEvent.bind(events), addEventListener: events.addEventListener.bind(events), removeEventListener: events.removeEventListener.bind(events), setInterval: () => 0 };
  t.after(() => Object.assign(global, previous));
}
function harness(file, api, initial = {}, data = {}, props = {}) {
  const states = [], refs = [], effects = [], routes = []; let index = 0, refIndex = 0;
  const stub = () => null;
  const hooks = {
    useState: init => { const i=index++; if (!(i in states)) states[i] = i in initial ? initial[i] : typeof init === 'function' ? init() : init; return [states[i], v => { states[i] = typeof v === 'function' ? v(states[i]) : v; }]; },
    useRef: init => { const i=refIndex++; return refs[i] ||= { current: init }; },
    useEffect: fn => effects.push(fn), useMemo: fn => fn(),
  };
  const Component = load(file, {
    react: hooks, 'next/navigation': { useRouter: () => ({ push: p => routes.push(p), replace: p => routes.push(p) }) },
    '@/lib/api-config': { getApiBaseUrl: () => '' }, '@/lib/user-api': api,
    '@/lib/customer-locale': { useCustomerLocale: () => ({ language: 'en', t: s => s, error: e => e instanceof Error ? e.message : e }) },
    '@/lib/pricing-plans': pricing, '@/components/pricing/PlanPricingCard': PlanCard,
    '@/components/public/PublicHeader': stub, '@/components/public/PublicFooter': stub,
    '@/components/public/useWebsiteSection': { useWebsiteSection: () => ({ enabled: true }) },
    '@/components/locale/CustomerLanguageSwitcher': stub, '@/components/brand/BrandLogo': stub,
    '@/lib/customer-payment-print': { printPaymentReceipt: async () => {} },
    '@/lib/razorpay-checkout': { loadRazorpayCheckout: async () => {} },
    './data': { useUserData: url => ({ data: data[url], reload() {}, loading: false, error: '' }) }, './Widgets': { LoadState: stub },
  }).default;
  return { routes, states, effects, data, render() { index=0; refIndex=0; return Component(props); } };
}
function cardButton(tree, id) {
  const card = find(tree, n => n.type === PlanCard && n.props.plan.id === id);
  assert.ok(card, 'plan is rendered');
  return find(PlanCard(card.props), n => n.type === 'button' && n.props.className?.includes('h-12'));
}
const noToken = { getUserToken: () => null, apiRequest: async () => ({}) };
const flush=()=>new Promise(resolve=>setImmediate(resolve));
test('visible paid pricing button uses upgrade action even with a stale CMS CTA URL', async t => {
  setup(t);
  const h=harness('app/pricing/page.tsx', noToken, {0:[professional],2:false});
  const button=cardButton(h.render(),10);
  assert.equal(typeof button?.props.onClick, 'function');
  await button.props.onClick();
  const url=new URL(h.routes[0], 'http://localhost');
  assert.equal(url.pathname, '/login/customer');
  assert.equal(url.searchParams.get('next'), '/dashboard/billing?plan=10&billing_period=monthly&currency=QAR&reviewed_amount=25.00');
});
test('free Get Started still opens selected-plan registration', async t => {
  setup(t);const h=harness('app/pricing/page.tsx', noToken, {0:[free],2:false});
  await cardButton(h.render(),31).props.onClick();
  assert.match(h.routes[0], /^\/register\?plan=31/);
});
test('logged-in pricing validates session and preserves annual selection', async t => {
  setup(t);const calls=[];const annual={...business,billing_period:'yearly'};
  const h=harness('app/pricing/page.tsx',{ getUserToken:()=> 'valid',apiRequest:async p=> {calls.push(p);return {id:1};} },{0:[annual],2:false,5:'yearly'});
  await cardButton(h.render(),14).props.onClick();
  assert.deepEqual(calls, ['/user/me']);
  assert.deepEqual(h.routes, ['/dashboard/billing?plan=14&billing_period=yearly&currency=SAR&reviewed_amount=100.00']);
});
test('login resumes a safe selected-plan destination after saving session', async t => {
  setup(t,{search:'?next=%2Fdashboard%2Fbilling%3Fplan%3D14%26billing_period%3Dyearly'});
  const tokens=[];const h=harness('app/login/customer/page.tsx',{...noToken,apiRequest:async()=>({token:'valid'}),saveUserToken:token=>tokens.push(token)},{0:'test@example.invalid',1:'password'});
  await find(h.render(), n=>n.type==='form').props.onSubmit({preventDefault(){}});
  assert.deepEqual(tokens,['valid']);assert.deepEqual(h.routes,['/dashboard/billing?plan=14&billing_period=yearly']);
});
test('login rejects external, encoded separator, and non-dashboard return URLs', () => {
  const { customerLoginDestination }=load('lib/customer-upgrade.ts');
  assert.equal(typeof customerLoginDestination,'function');
  for(const next of ['https://evil.invalid','//evil.invalid','/dashboard/../admin','/dashboard\\evil','/dashboard/%2f%2fevil','javascript:alert(1)']) assert.equal(customerLoginDestination(`?next=${encodeURIComponent(next)}`),'/dashboard');
});
test('registration and legacy login links preserve the selected plan and period',()=>{
  const {customerLoginDestination,customerRegistrationUrl}=load('lib/customer-upgrade.ts');
  assert.equal(customerLoginDestination('?plan=14&billing_period=yearly'),'/dashboard/billing?plan=14&billing_period=yearly');
  assert.equal(typeof customerRegistrationUrl,'function');
  assert.equal(customerRegistrationUrl('/dashboard/billing?plan=14&billing_period=yearly'),'/register?plan=14&billing_period=yearly');
});
test('zero-priced current plans compare below paid plans even in another currency',()=>{
  assert.equal(pricing.comparePlanPrices(professional,free),1);
  assert.equal(pricing.comparePlanPrices(free,professional),-1);
});
test('same tier annual switch is a billing-period change rather than a downgrade',()=>{
  assert.equal(typeof pricing.planChangeLabel,'function');
  assert.equal(pricing.planChangeLabel({...professional,id:11,price:240,billing_period:'yearly'},professional),'Change Billing Period');
  assert.equal(pricing.planChangeLabel(professional,free),'Upgrade Plan');
  assert.equal(pricing.planChangeLabel(free,professional),'Downgrade Plan');
});
test('double pricing clicks wait for one session check and report network errors',async t=>{
  setup(t);let finish,calls=0;
  const h=harness('app/pricing/page.tsx',{getUserToken:()=> 'valid',apiRequest:()=>{calls++;return new Promise((resolve,reject)=>{finish=reject;});}},{0:[business],2:false});
  const button=cardButton(h.render(),14);
  const first=button.props.onClick();await button.props.onClick();assert.equal(calls,1);
  assert.equal(cardButton(h.render(),14).props.disabled,true);
  finish(new TypeError('Failed to fetch'));await first;
  assert.equal(h.routes.length,0);assert.ok(find(h.render(),n=>n.props?.role==='alert'&&/session/.test(n.props.children)));
  assert.ok(!cardButton(h.render(),14).props.disabled);
});
test('expired customer session preserves upgrade selection through login',async t=>{
  setup(t);let token='expired';
  const h=harness('app/pricing/page.tsx',{getUserToken:()=>token,apiRequest:async()=>{token=null;throw Error('expired');}},{0:[business],2:false});
  await cardButton(h.render(),14).props.onClick();
  assert.equal(new URL(h.routes[0],'http://local').searchParams.get('next'),'/dashboard/billing?plan=14&billing_period=monthly&currency=SAR&reviewed_amount=100.00');
});
function billing(t, provider = { provider:'razorpay', supported_currencies:['QAR','SAR'] }, api, props={planID:'10',billingPeriod:'monthly'}) {
  setup(t);
  const calls=[];
  const h=harness('components/dashboard/Billing.tsx',{apiRequest:async(p,opts)=>{calls.push([p,opts]);return api ? api(p,opts) : provider;}},{},{'/user/dashboard':{},'/billing/history':{payments:[],current_subscription:null},'/website/pricing?lang=en':{data:[professional,business,free]},'/billing/provider':provider},props);
  h.calls=calls;return h;
}
test('trial plan checkout requires explicit agreement to pay now',async t=>{
 const h=billing(t);h.render();for(const fn of h.effects.splice(0))fn();
 find(h.render(),n=>n.type==='button'&&n.props.children==='Continue to checkout').props.onClick();await flush();
 assert.equal(h.calls.length,0,'no payment before agreement');
 assert.ok(find(h.render(),n=>n.props?.role==='alert'&&/agree to pay now/.test(n.props.children)));
});
test('starting the offered trial uses existing trial activation without payment',async t=>{
 const h=billing(t,undefined,async()=>({status:'trial'}));h.render();for(const fn of h.effects.splice(0))fn();
 const button=find(h.render(),n=>n.type==='button'&&n.props.children==='Start Free Trial');assert.ok(button,'eligible trial action');button.props.onClick();await flush();
 assert.deepEqual(h.calls.map(([p])=>p),['/billing/free/activate']);
});
test('billing opens requested plan confirmation without charging on mount', t => {
  const h=billing(t);h.render();for(const fn of h.effects.splice(0)) fn();
  assert.ok(find(h.render(),n=>n.props?.role==='dialog'),'selected plan confirmation is visible');assert.equal(h.calls.length,0);
});
test('dashboard paid action opens confirmation before checkout', async t => {
  const h=billing(t,undefined,undefined,{});
  await cardButton(h.render(),14).props.onClick();
  assert.ok(find(h.render(),n=>n.props?.role==='dialog'));assert.equal(h.calls.length,0);
});
function agreedCheckout(h){
 const consent=find(h.render(),n=>n.type==='input'&&n.props.type==='checkbox');
 if(consent)consent.props.onChange({target:{checked:true}});
 return find(h.render(),n=>n.type==='button'&&n.props.children==='Continue to checkout');
}
test('paid plans with trials create a payment order rather than free activation', async t => {
  let opened=0;
  const h=billing(t,undefined,(p)=>p==='/billing/checkout/order'?{key_id:'test',razorpay_order_id:'order_test',amount:2500,amount_major:'25',billing_period:'monthly',currency:'QAR'}:{provider:'razorpay',supported_currencies:['QAR']});
  window.Razorpay=class{on(){} open(){opened++;}};
  h.render();for(const fn of h.effects.splice(0))fn();
  const button=agreedCheckout(h);
  assert.ok(button,'explicit checkout confirmation');await button.props.onClick();await flush();
  assert.equal(opened,1,JSON.stringify({states:h.states,calls:h.calls}));assert.ok(h.calls.some(([p])=>p==='/billing/checkout/order'));assert.ok(!h.calls.some(([p])=>p==='/billing/free/activate'));
  const order=h.calls.find(([p])=>p==='/billing/checkout/order');assert.deepEqual(JSON.parse(order[1].body),{plan_id:10,billing_period:'monthly',currency:'QAR',country:'',expected_amount:'25.00',pay_now:true});
});
test('unsupported currency allows confirmation then gives actionable retry feedback', async t => {
  const provider={provider:'razorpay',supported_currencies:['INR']};
  const h=billing(t,provider,p=>{if(p==='/billing/checkout/order')throw Error('This currency is not enabled for the configured payment provider. Please contact support.');return provider;},{});
  const action=cardButton(h.render(),14);assert.ok(!action.props.disabled);await action.props.onClick();
  const confirm=agreedCheckout(h);assert.ok(confirm);await confirm.props.onClick();await flush();
  const alert=find(h.render(),n=>n.props?.role==='alert'&&/currency/.test(n.props.children));assert.ok(alert,'currency error displayed');
  assert.ok(h.calls.some(([p])=>p==='/billing/checkout/order'),'backend validates the authoritative pending order currency');
});

test('an updated checkout amount requires renewed confirmation',async t=>{
  let opened=0;const provider={provider:'razorpay',supported_currencies:['INR']};
  const h=billing(t,provider,p=>p==='/billing/checkout/order'?{key_id:'test',razorpay_order_id:'order_old',amount:10000,amount_major:'100',currency:'QAR',billing_period:'monthly'}:provider);
  window.Razorpay=class{on(){}open(){opened++;}};
  h.render();for(const fn of h.effects.splice(0))fn();
  await agreedCheckout(h).props.onClick();await flush();
  assert.equal(opened,0,'changed quote must not silently charge');
  assert.ok(find(h.render(),n=>n.props?.role==='alert'&&/total changed/.test(n.props.children)));
  await agreedCheckout(h).props.onClick();await flush();
  assert.equal(opened,1,'fresh confirmation resumes the allowed stored currency');
});
test('mismatched requested billing period gives an error without checkout', t => {
  const h=billing(t,undefined,undefined,{planID:'10',billingPeriod:'yearly'});h.render();for(const fn of h.effects.splice(0))fn();
  assert.ok(find(h.render(),n=>n.props?.role==='alert'&&/billing period/.test(n.props.children)));assert.equal(h.calls.length,0);
});
test('a paid plan trial can be converted to paid checkout for the same plan',async t=>{
  const h=billing(t,undefined,undefined,{});
  h.data['/billing/history'].current_subscription={plan_id:10,status:'trial',plan_name:'Professional',amount:0,currency:'QAR',billing_period:'monthly',starts_at:new Date().toISOString(),ends_at:null};
  const button=cardButton(h.render(),10);assert.ok(!button.props.disabled,'trial does not disable paid purchase');await button.props.onClick();assert.ok(find(h.render(),n=>n.props?.role==='dialog'));
});
test('checkout double confirmation creates one order and cancellation leaves the subscription unchanged',async t=>{
  let resolveProvider,options;const h=billing(t,undefined,(p)=>p==='/billing/provider'?new Promise(resolve=>{resolveProvider=resolve;}):{key_id:'test',razorpay_order_id:'order_test',amount:2500,amount_major:'25',billing_period:'monthly',currency:'QAR'});
  window.Razorpay=class {constructor(o){options=o;}on(){}open(){}};
  h.render();for(const fn of h.effects.splice(0))fn();const button=agreedCheckout(h);
  button.props.onClick();button.props.onClick();assert.equal(h.calls.length,1);
  resolveProvider({provider:'razorpay',supported_currencies:['QAR']});await flush();
  assert.equal(h.calls.filter(([p])=>p==='/billing/checkout/order').length,1);
  options.modal.ondismiss();
  assert.ok(find(h.render(),n=>n.props?.role==='status'&&/Checkout closed/.test(n.props.children)));assert.ok(!h.calls.some(([p])=>p==='/billing/checkout/verify'));
});


