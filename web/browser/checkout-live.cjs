// Real local customer and real Razorpay TEST checkout. No API/SDK fixtures.
const fs=require('node:fs'),assert=require('node:assert/strict'),{chromium}=require('playwright-core');
(async()=>{
 const account=JSON.parse(fs.readFileSync(process.env.CHECKOUT_TRIAL_CUSTOMER?'.feature-qa/trial-customer.json':'.feature-qa/upgrade-customer.json'));
 const planID=Number(process.env.CHECKOUT_PLAN_ID||10), expectedCurrency=process.env.CHECKOUT_CURRENCY||'INR',expectedAmount=Number(process.env.CHECKOUT_AMOUNT||2500);
 const browser=await chromium.launch({channel:'chrome',headless:true});
 let page, frame;const network=[];
 try{
  page=await browser.newPage({viewport:{width:1440,height:1000}});page.setDefaultTimeout(45000);
  page.on('response',r=>{const u=new URL(r.url());if(u.hostname.endsWith('razorpay.com'))network.push({host:u.hostname,path:u.pathname,status:r.status()})});
  page.on('requestfailed',r=>{const u=new URL(r.url());if(u.hostname.endsWith('razorpay.com'))network.push({host:u.hostname,path:u.pathname,error:r.failure()?.errorText})});
  await page.goto('http://localhost:3000/login/customer?next='+encodeURIComponent('/dashboard/billing?plan='+planID+'&billing_period=monthly'));
  await page.locator('input[type=email]').fill(account.email);await page.locator('input[autocomplete=current-password]').fill(account.password);await page.getByRole('button',{name:'Sign In',exact:true}).click();await page.waitForURL('**/dashboard/billing?**');
  const dialog=page.getByRole('dialog');await dialog.waitFor();
  if(process.env.CHECKOUT_START_FREE){
   await dialog.getByRole('button',{name:'Close',exact:true}).click();
   const free=page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name:'Normal Users',exact:true})});await free.getByRole('button',{name:'Get Started',exact:true}).click();await dialog.waitFor();await dialog.getByRole('button',{name:'Get Started',exact:true}).click();await page.getByRole('status').filter({hasText:'Your free plan is active.'}).waitFor();await free.getByRole('button',{name:'Current Plan',exact:true}).waitFor();
   await page.goto('http://localhost:3000/pricing',{waitUntil:'domcontentloaded'});await page.locator('[data-pricing-card]').filter({has:page.getByRole('heading',{name:'Professional',exact:true})}).getByRole('button',{name:'Upgrade Plan',exact:true}).click();await page.waitForURL('**/dashboard/billing?**');await dialog.waitFor();
  }
  const consent=dialog.locator('input[type=checkbox]');if(await consent.count())await consent.check();
  const response=page.waitForResponse(r=>r.url().endsWith('/billing/checkout/order'));await dialog.getByRole('button',{name:'Continue to checkout'}).click();
  const r=await response,order=await r.json();assert.ok([200,201].includes(r.status()),JSON.stringify(order));assert.equal(order.mode,'test');assert.ok(order.key_id.startsWith('rzp_test_'));assert.equal(order.amount,expectedAmount);assert.equal(order.currency,expectedCurrency);
  const result={endpoint:r.url(),status:r.status(),request:r.request().postDataJSON(),customer_id:account.id,order_id:order.razorpay_order_id,amount:order.amount,currency:order.currency,mode:order.mode,started_from_free:!!process.env.CHECKOUT_START_FREE};
  await page.locator('iframe.razorpay-checkout-frame').waitFor({state:'visible'});
  result.checkout_visible=true;
  frame=await (await page.locator('iframe.razorpay-checkout-frame').elementHandle()).contentFrame();
  await frame.waitForFunction(()=>!!document.body&&document.body.innerText.trim().length>30);
  result.checkout_text=(await frame.locator('body').innerText()).slice(0,3500);
  result.controls=await frame.locator('input,button').evaluateAll(nodes=>nodes.map(n=>({tag:n.tagName,type:n.type,name:n.name,placeholder:n.placeholder,text:n.innerText})).slice(0,45));
  if(process.env.CHECKOUT_OUTCOME==='cancel'){
   await frame.locator('input[name=contact]').fill('9000090000');await frame.getByRole('button',{name:'Continue',exact:true}).last().click();await frame.locator('input[name=contact]').waitFor({state:'hidden'});
   console.log('Header buttons '+JSON.stringify(await frame.locator('button[title]').evaluateAll(ns=>ns.map(n=>({title:n.title,testid:n.dataset.testid})))));
   await frame.getByTitle(/close/i).first().click();
   await frame.getByRole('button',{name:'Yes, exit',exact:true}).click();
   await page.getByRole('status').filter({hasText:'Checkout closed. No subscription change has been made.'}).waitFor();
   result.outcome='customer cancellation';result.billing=await page.evaluate(async()=>fetch('http://localhost:8080/api/v1/billing/history',{headers:{Authorization:'Bearer '+localStorage.getItem('user_token')}}).then(r=>r.json()));assert.equal(result.billing.current_subscription.plan_id,10);assert.equal(result.billing.payments.filter(p=>p.status==='paid').length,1);
  }else if(process.env.CHECKOUT_OUTCOME==='upi-inspect'){
   await frame.locator('input[name=contact]').fill('9000090000');await frame.getByRole('button',{name:'Continue',exact:true}).last().click();await frame.locator('input[name=contact]').waitFor({state:'hidden'});await frame.getByText('UPI',{exact:true}).first().click();
   result.payment_step=await frame.locator('body').innerText();result.payment_controls=await frame.locator('input,button').evaluateAll(ns=>ns.map(n=>({tag:n.tagName,type:n.type,name:n.name,placeholder:n.placeholder,text:n.innerText,title:n.title,testid:n.dataset.testid})));
  }else if(process.env.CHECKOUT_OUTCOME==='card-failure'){
   await frame.locator('input[name=contact]').fill('9000090000');await frame.getByRole('button',{name:'Continue',exact:true}).last().click();await frame.locator('input[name=contact]').waitFor({state:'hidden'});
   await frame.getByText('Cards',{exact:true}).click();await frame.locator('input[name="card.number"]').pressSequentially('4100280000080001',{delay:60});await frame.locator('input[name="card.expiry"]').pressSequentially('1229',{delay:60});await frame.locator('input[name="card.cvv"]').pressSequentially('123',{delay:60});
   await frame.locator('input[name="card.cvv"]').press('Tab');
   const popup=page.waitForEvent('popup').then(bank=>({bank}),error=>({error}));await frame.locator('input[name="card.cvv"]').press('Enter');
   await frame.getByRole('button',{name:'Maybe later',exact:true}).click();
   const pending=await popup;if(pending.error)throw pending.error;const bank=pending.bank;await bank.waitForLoadState('domcontentloaded');
   await bank.waitForFunction(()=>document.body.innerText.includes('OTP')||document.body.innerText.includes('Failure'));
   console.log('Test bank controls '+JSON.stringify(await bank.locator('input,button').evaluateAll(nodes=>nodes.map(n=>({tag:n.tagName,type:n.type,name:n.name,placeholder:n.placeholder,text:n.innerText})))));
   if(await bank.getByRole('button',{name:'Failure',exact:true}).count())await bank.getByRole('button',{name:'Failure',exact:true}).click();
   else {await bank.locator('input').first().fill('12');await bank.getByRole('button',{name:/Submit/}).click();}
   await page.getByRole('alert').filter({hasText:'Payment failed.'}).waitFor();
   result.outcome='provider failure';result.billing=await page.evaluate(async()=>fetch('http://localhost:8080/api/v1/billing/history',{headers:{Authorization:'Bearer '+localStorage.getItem('user_token')}}).then(r=>r.json()));assert.equal(result.billing.current_subscription.status,'trial');assert.equal(result.billing.payments.filter(p=>p.status==='paid').length,0);
  }else if(process.env.CHECKOUT_OUTCOME){
   await frame.locator('input[name=contact]').fill('9000090000');await frame.getByRole('button',{name:'Continue',exact:true}).click();
   console.log('Contact step: '+(await frame.locator('body').innerText()).slice(-1800));
   if(process.env.CHECKOUT_OUTCOME!=='inspect'){
    await frame.locator('input[name=contact]').waitFor({state:'hidden'});
    console.log('Netbanking control: '+await frame.getByText('Netbanking',{exact:true}).evaluate(n=>n.parentElement.outerHTML));
    await frame.getByText('Netbanking',{exact:true}).click();
   }
   if(process.env.CHECKOUT_OUTCOME!=='inspect'){
    const popupPromise=page.waitForEvent('popup');
    await frame.getByText('Canara Bank',{exact:true}).first().click();
    const bank=await popupPromise;await bank.waitForLoadState('domcontentloaded');
    bank.on('requestfailed',r=>network.push({host:new URL(r.url()).hostname,path:new URL(r.url()).pathname,error:r.failure()?.errorText}));
    if(process.env.CHECKOUT_OUTCOME==='success'){
     // Some TEST banks finish automatically; others show Razorpay's mock-bank button.
     void bank.getByRole('button',{name:'Success',exact:true}).waitFor({state:'visible'}).then(()=>bank.getByRole('button',{name:'Success',exact:true}).click()).catch(()=>{});
    }
    if(['success','failure'].includes(process.env.CHECKOUT_OUTCOME)){
     if(process.env.CHECKOUT_OUTCOME==='failure')await bank.getByRole('button',{name:'Failure',exact:true}).click();
     if(process.env.CHECKOUT_OUTCOME==='success'){
      await page.getByRole('status').filter({hasText:'Payment successful. Your plan has been activated.'}).waitFor();
      result.activation='verified';
     }else{
      await page.getByRole('alert').filter({hasText:'Payment failed.'}).waitFor();result.activation='none';
     }
     result.billing=await page.evaluate(async()=>{const r=await fetch('http://localhost:8080/api/v1/billing/history',{headers:{Authorization:'Bearer '+localStorage.getItem('user_token')}});if(!r.ok)throw Error('Billing history failed');return r.json()});
    }
   }
   if(!result.activation){result.payment_step=(await frame.locator('body').innerText()).slice(0,3500);}
  }
  const scenario=expectedCurrency+(process.env.CHECKOUT_OUTCOME?'-'+process.env.CHECKOUT_OUTCOME:'');
  fs.writeFileSync('artifacts/upgrade/checkout-'+scenario+'.json',JSON.stringify(result,null,2));await page.screenshot({path:'artifacts/upgrade/checkout-'+scenario+'.png',timeout:15000});console.log(JSON.stringify(result));
 }catch(e){
  fs.writeFileSync('artifacts/upgrade/checkout-failure.json',JSON.stringify({error:e.message,network,text:frame?await frame.locator('body').innerText().catch(()=>null):null},null,2));
  if(page)await page.screenshot({path:'artifacts/upgrade/checkout-failure.png'}).catch(()=>{});throw e;
 }finally{await browser.close();}
})().catch(e=>{console.error(e.message);process.exitCode=1});


