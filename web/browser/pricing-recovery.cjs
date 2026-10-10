const {test,before,after}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const {chromium}=require('playwright-core');
let browser;
before(async()=>{browser=await chromium.launch({channel:'chrome',headless:true});await fs.mkdir('artifacts/pricing-recovery',{recursive:true});});
after(async()=>browser?.close());
test('real pricing keeps only currency selection and retains all cards without request loops',async()=>{
 const page=await browser.newPage();const responses=[];
 page.on('response',r=>{if(new URL(r.url()).pathname==='/api/v1/website/pricing')responses.push(r)});
 await page.addInitScript(()=>localStorage.setItem('dbizcard.billing.currency',JSON.stringify({currency:'SAR',country:'SA'})));
 await page.goto('http://localhost:3000/pricing');
 await page.locator('[data-pricing-card]').first().waitFor();
 for(const currency of ['SAR','INR','AED','QAR','USD','EUR','GBP']){
  await page.getByRole('combobox',{name:'Currency',exact:true}).selectOption(currency);
  for(const name of ['Normal Users','Professional','Business'])await page.locator('[data-pricing-card]').getByRole('heading',{name,exact:true}).waitFor();
  assert.equal(await page.locator('[data-pricing-card]').count(),4);
  assert.equal(await page.getByRole('combobox',{name:'Currency',exact:true}).inputValue(),currency);
 }
 assert.equal(await page.getByRole('searchbox').count(),0);
 assert.equal(await page.getByRole('combobox',{name:'Country',exact:true}).count(),0);
 assert.equal(await page.getByRole('button',{name:/Suggested currency/}).count(),0);
 assert.doesNotMatch(await page.locator('main').innerText(),/Changing country|Search countries|Country name or code/);
 await page.getByRole('combobox',{name:'Currency',exact:true}).selectOption('INR');
 assert.equal(await page.getByRole('combobox',{name:'Currency',exact:true}).inputValue(),'INR');
 const business=page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name:'Business',exact:true})});
 assert.match(await business.innerText(),/₹200/);
 const professional=page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name:'Professional',exact:true})});
 assert.match(await professional.innerText(),/Pricing unavailable in INR/);
 assert.ok(await professional.getByRole('button',{name:'Upgrade Plan',exact:true}).isDisabled());
 assert.ok(await page.getByRole('button',{name:'Yearly',exact:true}).isDisabled(),'do not invent yearly prices');
 assert.equal(await page.getByRole('combobox',{name:'Currency',exact:true}).inputValue(),'INR');
 const successful=responses.filter(r=>r.status()===200);assert.ok(successful.length>=1);
 assert.ok(responses.length<=2,'currency changes do not refetch the catalog');
 await page.screenshot({path:'artifacts/pricing-recovery/inr-cards.png',fullPage:true});
 await fs.writeFile('artifacts/pricing-recovery/network.json',JSON.stringify(await Promise.all(successful.map(async r=>({url:r.url(),status:r.status(),body:await r.json()}))),null,2));
 await business.getByRole('button',{name:'Upgrade Plan',exact:true}).click();
 await page.waitForURL(url=>url.pathname==='/login/customer');
 const destination=new URL(new URL(page.url()).searchParams.get('next'),'http://localhost:3000');
 assert.equal(destination.searchParams.get('currency'),'INR');
 assert.equal(destination.searchParams.has('country'),false);
 await page.close();
});

test('billing fixture verifies periods, exact checkout currency, free activation and independent history',async()=>{
 const page=await browser.newPage();const calls=[];const errors=[];
 page.on('pageerror',error=>errors.push(error.message));
 await page.addInitScript(()=>{
  localStorage.setItem('user_token','pricing-recovery-fixture');
  localStorage.setItem('dbizcard.billing.currency',JSON.stringify({currency:'SAR',country:'SA'}));
  window.Razorpay=class{constructor(options){window.__pricingCheckout=options}on(){}open(){window.__pricingOpened=true}};
 });
 const plans=[
  {id:10,name:'Professional',price:100,currency:'INR',billing_period:'monthly',pricing_type:'fixed',trial_days:0,trial_mode:'disabled',billing_config:{providers:['razorpay'],prices:[{currency:'SAR',country:'',billing_period:'monthly',amount:'20',tax_bps:0},{currency:'SAR',country:'',billing_period:'yearly',amount:'200',tax_bps:0}]}},
  {id:14,name:'Business',price:200,currency:'INR',billing_period:'monthly',pricing_type:'fixed',trial_days:0},
  {id:31,name:'Normal Users',price:0,currency:'INR',billing_period:'lifetime',pricing_type:'free',trial_days:0}
 ];
 let current=null,mismatch=true;
 const historyPayment={payment_public_id:'fixture-history',plan_id:10,plan_name:'Professional',amount:20,currency:'SAR',provider:'razorpay',status:'paid',payment_method:'upi',created_at:'2026-10-09T00:00:00Z',paid_at:'2026-10-09T00:00:00Z'};
 await page.route('**/api/v1/**',async route=>{
  const req=route.request(),endpoint=new URL(req.url()).pathname.replace('/api/v1','');
  if(!['/user/me','/user/dashboard','/billing/history','/billing/provider','/website/pricing','/billing/checkout/order','/billing/checkout/verify','/billing/free/activate'].includes(endpoint))return route.continue();
  const body=req.postDataJSON();calls.push({endpoint,body});
  let data={};
  if(endpoint==='/user/me')data={id:1,name:'Fixture',email:'fixture@example.invalid',status:'active'};
  if(endpoint==='/user/dashboard')data={total_cards:0,published_cards:0,current_plan:current};
  if(endpoint==='/billing/history')data={payments:[historyPayment],subscriptions:current?[current]:[],current_subscription:current};
  if(endpoint==='/billing/provider')data={provider:'razorpay',mode:'test',supported_currencies:['INR','SAR']};
  if(endpoint==='/website/pricing')data={data:plans};
  if(endpoint==='/billing/checkout/order')data={key_id:'rzp_test_fixture',razorpay_order_id:'order_fixture',amount:10000,amount_major:'100.00',billing_period:'monthly',currency:mismatch?'SAR':'INR',prefill:{name:'Fixture',email:'fixture@example.invalid'}};
  if(endpoint==='/billing/checkout/verify'){current={plan_id:10,plan_name:'Professional',status:'active',amount:100,currency:'INR',billing_period:'monthly',starts_at:'2026-10-09T00:00:00Z'};data={status:'paid',subscription_id:1};}
  if(endpoint==='/billing/free/activate'){current={plan_id:31,plan_name:'Normal Users',status:'active',amount:0,currency:'INR',billing_period:'lifetime',starts_at:'2026-10-09T00:00:00Z'};data={status:'active'};}
  await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(data)});
 });
 await page.goto('http://localhost:3000/pricing');
 const card=name=>page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name,exact:true})});
 await card('Professional').waitFor();

 await page.getByRole('button',{name:/^Yearly/}).click();
 await card('Professional').getByText(/200/).first().waitFor();
 assert.equal(await page.getByRole('combobox',{name:'Currency',exact:true}).inputValue(),'SAR');
 await page.getByRole('button',{name:'Monthly',exact:true}).click();
 await page.getByRole('combobox',{name:'Currency',exact:true}).selectOption('INR');
 await card('Professional').getByRole('button',{name:'Upgrade Plan',exact:true}).click();
 await page.waitForURL(url=>url.pathname==='/dashboard/billing');
 assert.equal(new URL(page.url()).searchParams.get('currency'),'INR');
 assert.equal(new URL(page.url()).searchParams.has('country'),false);
 await page.getByRole('heading',{name:'Payment / Billing History',exact:true}).waitFor();
 await page.getByRole('cell',{name:'fixture-history',exact:true}).waitFor();
 const dialog=page.getByRole('dialog');await dialog.waitFor();
 assert.match(await dialog.innerText(),/INR 100.00/);
 await dialog.getByRole('button',{name:'Continue to checkout',exact:true}).click();
 await dialog.getByRole('alert').filter({hasText:'different currency'}).waitFor();
 assert.equal(await page.evaluate(()=>window.__pricingOpened),undefined,'mismatched response cannot open gateway');
 mismatch=false;
 await dialog.getByRole('button',{name:'Continue to checkout',exact:true}).click();
 await page.waitForFunction(()=>window.__pricingOpened===true);
 const options=await page.evaluate(()=>({currency:window.__pricingCheckout.currency,amount:window.__pricingCheckout.amount,method:window.__pricingCheckout.method}));
 assert.deepEqual(options,{currency:'INR',amount:10000,method:undefined},'unconfigured methods remain native, without cards-only restriction');
 assert.ok(calls.filter(c=>c.endpoint==='/billing/checkout/order').every(c=>c.body.currency==='INR'&&c.body.expected_amount==='100.00'&&c.body.billing_period==='monthly'&&c.body.country===''));
 await page.evaluate(()=>window.__pricingCheckout.handler({razorpay_order_id:'order_fixture',razorpay_payment_id:'pay_fixture',razorpay_signature:'fixture'}));
 await page.getByRole('status').filter({hasText:'Payment successful'}).waitFor();
 await card('Normal Users').getByRole('button',{name:'Downgrade Plan',exact:true}).click();
 await dialog.getByRole('button',{name:'Get Started',exact:true}).click();
 await page.getByRole('status').filter({hasText:'Your free plan is active.'}).waitFor();
 assert.equal(calls.filter(c=>c.endpoint==='/billing/free/activate').length,1);
 assert.equal(calls.filter(c=>c.endpoint==='/billing/checkout/order').length,2);
 assert.deepEqual(errors,[]);
 await page.close();
});

test('real Arabic pricing retains cards with only localized currency selection',async()=>{
 const page=await browser.newPage();
 await page.addInitScript(()=>{localStorage.setItem('frontend_language','ar');localStorage.setItem('dbizcard.billing.currency',JSON.stringify({currency:'SAR',country:'SA'}));});
 await page.goto('http://localhost:3000/pricing');
 await page.getByRole('combobox',{name:'العملة',exact:true}).waitFor();
 await page.locator('[data-pricing-card]').first().waitFor();
 assert.equal(await page.locator('[data-pricing-card]').count(),4);
 assert.equal(await page.getByRole('combobox',{name:'البلد',exact:true}).count(),0);
 assert.equal(await page.getByRole('searchbox').count(),0);
 assert.match(await page.locator('main').innerText(),/السعر غير متاح بـ SAR/);
 await page.screenshot({path:'artifacts/pricing-recovery/arabic-sar-cards.png',fullPage:true});
 await page.close();
});

test('mobile pricing shows a responsive currency-only selector',async()=>{
 const page=await browser.newPage({viewport:{width:390,height:844},isMobile:true,hasTouch:true});
 await page.addInitScript(()=>localStorage.setItem('dbizcard.billing.currency',JSON.stringify({currency:'INR',country:'US'})));
 await page.goto('http://localhost:3000/pricing');
 await page.locator('[data-pricing-card]').first().waitFor();
 const select=page.getByRole('combobox',{name:'Currency',exact:true});
 assert.equal(await select.inputValue(),'INR');
 assert.ok(await select.locator('option').count()>150,'configured currency catalog is retained');
 assert.equal(await page.getByRole('searchbox').count(),0);
 assert.equal(await page.getByRole('combobox',{name:'Country',exact:true}).count(),0);
 assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=window.innerWidth),'no horizontal overflow');
 await select.selectOption('SAR');
 await page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name:'Professional',exact:true})}).getByText(/100/).first().waitFor();
 await page.screenshot({path:'artifacts/pricing-recovery/mobile-currency-only.png',fullPage:true});
 await page.close();
});
