// Browser regression tests against the running frontend. Public pricing uses the
// real Go API; authenticated component scenarios below explicitly use fixtures.
const { test, before, after } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const { chromium } = require('playwright-core');
const base = process.env.UPGRADE_BASE_URL || 'http://localhost:3000';
let browser;
before(async () => { browser = await chromium.launch({ channel: 'chrome', headless: true }); await fs.mkdir('artifacts/upgrade', { recursive: true }); });
after(async () => { await browser?.close(); });
async function session(t, language, mobile) {
  const context = await browser.newContext({ viewport: mobile ? { width:390,height:844 } : { width:1440,height:1000 }, isMobile:mobile,hasTouch:mobile });
  await context.addInitScript(language => localStorage.setItem('frontend_language', language), language);
  const page = await context.newPage(); page.setDefaultTimeout(60000); page.setDefaultNavigationTimeout(60000); const events=[], errors=[];
  page.on('pageerror',e => errors.push(e.message));
  page.on('console',m => {if(m.type()==='error')events.push({type:'console',text:m.text()});});
  page.on('response',r => {if(r.url().includes('/api/v1/'))events.push({type:'response',url:r.url(),status:r.status()});});
  page.on('requestfailed',r=>events.push({type:'requestfailed',url:r.url(),error:r.failure()?.errorText}));
  t.after(async()=>{const name=t.name.replace(/[^a-z0-9]+/gi,'-');await fs.writeFile(path.join('artifacts/upgrade',name+'.json'),JSON.stringify({events,errors,url:page.url()},null,2));try{await page.screenshot({path:path.join('artifacts/upgrade',name+'.png'),animations:'disabled',timeout:15000});}finally{await context.close();}assert.deepEqual(errors,[],'no runtime exceptions');assert.ok(!events.some(e=>e.type==='console'&&/hydration|Minified React/.test(e.text)),'no hydration errors');});
  return page;
}
for(const language of ['en','ar']) for(const mobile of [false,true]) test(`real pricing paid click opens customer login ${language} ${mobile?'mobile':'desktop'}`,async t=>{
  const page=await session(t,language,mobile);
  await page.goto(base+'/pricing',{waitUntil:'domcontentloaded'});
  const card=page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name:language==='ar'?'احترافي':'Professional',exact:true})});
  await card.locator('button').filter({hasText:language==='ar'?'ترقية':'Upgrade Plan'}).first().click();
  await page.waitForURL(u=>u.pathname==='/login/customer');
  assert.equal(new URL(page.url()).searchParams.get('next'),'/dashboard/billing?plan=10&billing_period=monthly');
  await page.getByRole('heading',{name:language==='ar'?'دخول العميل':'Customer Login',exact:true}).waitFor({state:'visible'});
});

async function fixtures(page, { currencies=['QAR','SAR'], yearly=false, loggedIn=true, orderError=false }={}) {
  // Only this context receives test fixtures. No customer or payment DB is mutated.
  const plans=[{id:10,name:'Professional',price:25,currency:'QAR',billing_period:'monthly',pricing_type:'fixed',trial_mode:'limited',trial_days:20,description:'',feature_limits:{features:['analytics'],limits:{digital_cards_limit:3}}},{id:14,name:'Business',price:100,currency:'SAR',billing_period:'monthly',pricing_type:'fixed',trial_days:100,trial_mode:'limited',description:'',feature_limits:{features:['analytics'],limits:{digital_cards_limit:10}}},{id:31,name:'Normal Users',price:0,currency:'SAR',billing_period:'lifetime',pricing_type:'free',trial_days:0,description:'',feature_limits:{features:[],limits:{digital_cards_limit:1}}}];
  if(yearly)plans.push({...plans[1],id:15,price:1000,billing_period:'yearly'});
  let current={plan_id:31,plan_name:'Normal Users',status:'active',amount:0,currency:'SAR',billing_period:'lifetime',starts_at:'2026-10-01T00:00:00Z',ends_at:null};
  let payments=[];const calls=[];
  await page.addInitScript(loggedIn=>{if(loggedIn)localStorage.setItem('user_token','browser-fixture');window.Razorpay=class{constructor(options){this.options=options;window.__fixtureCheckout=this;this.events={};}on(name,fn){this.events[name]=fn;}open(){window.__fixtureCheckoutOpened=true;}};},loggedIn);
  await page.route('**/api/v1/**',async route=>{
    const req=route.request();const url=new URL(req.url());const endpoint=url.pathname.replace('/api/v1','');
    if(!endpoint.startsWith('/user/')&&!endpoint.startsWith('/billing/')&&endpoint!='/auth/login'&&endpoint!='/website/pricing')return route.continue();
    const body=req.postDataJSON();calls.push({endpoint,body,authorization:req.headers().authorization});
    let data,status=200;
    if(endpoint==='/auth/login')data={token:'browser-fixture'};
    else if(endpoint==='/user/me')data={id:1,public_id:'fixture',name:'Browser Fixture',email:'fixture@example.invalid',status:'active'};
    else if(endpoint==='/website/pricing')data={data:plans};
    else if(endpoint==='/billing/provider')data={provider:'razorpay',mode:'test',supported_currencies:currencies};
    else if(endpoint==='/billing/history')data={current_subscription:current,subscriptions:[current],payments};
    else if(endpoint==='/user/dashboard')data={total_cards:0,published_cards:0,draft_cards:0,total_contacts:0,total_qr_scans:0,total_nfc_taps:0,current_plan:{...current,name:current.plan_name,features:[],limits:{}}};
    else if(endpoint==='/billing/checkout/order'){
      const plan=plans.find(p=>p.id===body.plan_id);
      if(!currencies.includes(plan.currency)){status=400;data={error:'This currency is not enabled for the configured Razorpay merchant account. Confirm merchant activation and update the backend currency allowlist.'};}
      else if(orderError){status=502;data={error:'Unable to start payment. Please try again.'};}
      else data={key_id:'rzp_test_browser_fixture',razorpay_order_id:'order_browser_fixture',payment_public_id:'payment_browser_fixture',amount:plan.price*100,amount_major:String(plan.price),billing_period:plan.billing_period,currency:plan.currency,plan_name:plan.name,prefill:{name:'Browser Fixture',email:'fixture@example.invalid'}};
    }else if(endpoint==='/billing/checkout/verify'){
      const selected=calls.findLast(c=>c.endpoint==='/billing/checkout/order').body;
      const plan=plans.find(p=>p.id===selected.plan_id);
      current={...current,plan_id:plan.id,plan_name:plan.name,amount:plan.price,currency:plan.currency,billing_period:plan.billing_period};
      payments=[{plan_id:plan.id,plan_name:plan.name,payment_public_id:'payment_browser_fixture',amount:plan.price,currency:plan.currency,provider:'razorpay',status:'paid',payment_method:'card',paid_at:'2026-10-09T00:00:00Z',created_at:'2026-10-09T00:00:00Z'}];
      data={status:'paid'};
    }else data={data:[],pagination:{total:0,pages:1,page:1}};
    await route.fulfill({status,contentType:'application/json',headers:{'access-control-allow-origin':'*'},body:JSON.stringify(data)});
  });
  return {calls};
}
for(const language of ['en','ar']) for(const mobile of [false,true]) test(`fixture authenticated paid click opens confirmation and currency error ${language} ${mobile?'mobile':'desktop'}`,async t=>{
  const page=await session(t,language,mobile);const h=await fixtures(page,{currencies:['INR']});
  await page.goto(base+'/pricing',{waitUntil:'domcontentloaded'});
  const card=page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name:language==='ar'?'احترافي':'Professional',exact:true})});
  await card.getByRole('button',{name:language==='ar'?'ترقية الخطة':'Upgrade Plan',exact:true}).click();
  await page.waitForURL(u=>u.pathname==='/dashboard/billing');
  const dialog=page.getByRole('dialog');await dialog.waitFor();
  await dialog.locator('input[type=checkbox]').check();await dialog.getByRole('button',{name:language==='ar'?'المتابعة إلى الدفع':'Continue to checkout'}).click();
  await dialog.getByRole('alert').filter({hasText:language==='ar'?'هذه العملة':'This currency'}).waitFor();
  assert.equal(h.calls.filter(c=>c.endpoint==='/billing/checkout/order').length,1);
  assert.ok(h.calls.some(c=>c.endpoint==='/user/me'&&c.authorization==='Bearer browser-fixture'));
});
test('fixture login resumes annual Business confirmation and verified fixture updates billing',async t=>{
  const page=await session(t,'en',false);const h=await fixtures(page,{yearly:true,loggedIn:false});
  await page.goto(base+'/pricing',{waitUntil:'domcontentloaded'});await page.getByRole('button',{name:/Yearly/}).click();
  const card=page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name:'Business',exact:true})});
  await card.getByRole('button',{name:'Upgrade Plan',exact:true}).click();await page.waitForURL(u=>u.pathname==='/login/customer');
  await page.locator('input[type=email]').fill('fixture@example.invalid');await page.locator('input[autocomplete=current-password]').fill('fixture-only');await page.getByRole('button',{name:'Sign In',exact:true}).click();
  await page.waitForURL(u=>u.pathname==='/dashboard/billing');assert.equal(new URL(page.url()).searchParams.get('billing_period'),'yearly');
  const dialog=page.getByRole('dialog');await dialog.waitFor();await dialog.locator('input[type=checkbox]').check();await dialog.getByRole('button',{name:'Continue to checkout'}).click();
  await page.waitForFunction(()=>window.__fixtureCheckoutOpened===true);
  assert.deepEqual(h.calls.find(c=>c.endpoint==='/billing/checkout/order').body,{plan_id:15,billing_period:'yearly',pay_now:true});
  await page.evaluate(()=>window.__fixtureCheckout.options.handler({razorpay_order_id:'order_browser_fixture',razorpay_payment_id:'pay_browser_fixture',razorpay_signature:'fixture-only'}));
  await page.getByRole('status').filter({hasText:'Payment successful. Your plan has been activated.'}).waitFor();
  await page.locator('section').filter({has:page.getByRole('heading',{name:'Current Plan',exact:true})}).getByText('Business',{exact:true}).waitFor();
  assert.equal(h.calls.filter(c=>c.endpoint==='/billing/checkout/verify').length,1);
  assert.equal(h.calls.filter(c=>c.endpoint==='/billing/free/activate').length,0);
});
test('fixture provider error restores confirmation for retry without activation',async t=>{
  const page=await session(t,'en',true);const h=await fixtures(page,{orderError:true});
  await page.goto(base+'/dashboard/billing?plan=14&billing_period=monthly');const dialog=page.getByRole('dialog');await dialog.waitFor();
  await dialog.locator('input[type=checkbox]').check();const button=dialog.getByRole('button',{name:'Continue to checkout'});await button.click();
  await dialog.getByRole('alert').filter({hasText:'Unable to start payment'}).waitFor();assert.ok(await button.isEnabled());
  assert.equal(h.calls.filter(c=>c.endpoint==='/billing/checkout/verify').length,0);
});
test('real expired billing session preserves requested plan when redirecting to login',async t=>{
  const page=await session(t,'en',false);await page.goto(base+'/dashboard/billing?plan=14&billing_period=monthly');
  await page.waitForURL(u=>u.pathname==='/login/customer');assert.equal(new URL(page.url()).searchParams.get('next'),'/dashboard/billing?plan=14&billing_period=monthly');
});
test('real free pricing registration still preserves Normal Users',async t=>{
  const page=await session(t,'en',true);await page.goto(base+'/pricing',{waitUntil:'domcontentloaded'});
  const card=page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name:'Normal Users',exact:true})});
  await card.getByRole('button',{name:'Get Started',exact:true}).click();await page.waitForURL(u=>u.pathname==='/register');assert.equal(new URL(page.url()).searchParams.get('plan'),'31');
});


