// Read-only audit of the genuine TEST payment completed by Razorpay Checkout.
const fs=require('node:fs'),assert=require('node:assert/strict'),{chromium}=require('playwright-core');
(async()=>{const a=JSON.parse(fs.readFileSync('.feature-qa/upgrade-customer.json'));const browser=await chromium.launch({channel:'chrome',headless:true});try{
 const page=await browser.newPage();await page.goto('http://localhost:3000/login/customer?next=%2Fdashboard%2Fbilling');await page.locator('input[type=email]').fill(a.email);await page.locator('input[autocomplete=current-password]').fill(a.password);await page.getByRole('button',{name:'Sign In',exact:true}).click();await page.waitForURL('**/dashboard/billing');
 await page.getByRole('heading',{name:'Current Plan',exact:true}).waitFor();
 const data=await page.evaluate(async()=>{const headers={Authorization:'Bearer '+localStorage.getItem('user_token')};const billing=await fetch('http://localhost:8080/api/v1/billing/history',{headers}).then(r=>r.json());const dashboard=await fetch('http://localhost:8080/api/v1/user/dashboard',{headers}).then(r=>r.json());return {billing,dashboard}});
 assert.equal(data.billing.current_subscription.plan_id,10);assert.equal(data.billing.current_subscription.status,'active');const paid=data.billing.payments.filter(p=>p.status==='paid');assert.equal(paid.length,1);assert.equal(paid[0].currency,'INR');assert.equal(Number(paid[0].amount),25);
 await page.locator('section').filter({has:page.getByRole('heading',{name:'Current Plan',exact:true})}).getByText('Professional',{exact:true}).waitFor();
 data.visible_text=await page.locator('main').innerText();fs.writeFileSync('artifacts/upgrade/checkout-activation.json',JSON.stringify(data,null,2));await page.screenshot({path:'artifacts/upgrade/checkout-activation.png',fullPage:true});console.log(JSON.stringify(data));
}finally{await browser.close()}})().catch(e=>{console.error(e.message);process.exitCode=1});
