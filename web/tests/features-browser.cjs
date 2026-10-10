// Run against a built server with playwright-core already installed.
// The catalogue must match the public CMS API, including a genuinely empty list.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright-core');
const assert = require('node:assert/strict');
(async () => {
 const browser = await chromium.launch({ executablePath: process.env.CHROME_PATH || 'C:/Program Files/Google/Chrome/Application/chrome.exe', headless: true });
 try {
  const page=await browser.newPage();const errors=[];page.on('pageerror',error=>errors.push(error.message));
  const url=process.env.FEATURES_URL || 'http://localhost:3100/features';
  for(const width of [320,768,1440]) {
   await page.setViewportSize({width,height:900});
   const responsePromise=page.waitForResponse(r=>r.url().endsWith('/public/website/features'));
   await page.goto(url);const response=await responsePromise;
   if(response.status()===404){await page.locator('main').waitFor({state:'detached'});continue;}
   assert(response.ok());const data=await response.json();
   const cards=page.locator('section[aria-label="Featured product capabilities"] article');
   const expected=data.items.filter(item=>item.is_enabled!==false);
   if(expected.length){await cards.first().waitFor();assert.equal(await cards.count(),expected.length);for(const item of expected)assert(await cards.getByRole('heading',{name:item.title,exact:true}).count()>0);}
   else {await page.getByText('No items found',{exact:true}).waitFor();assert.equal(await cards.count(),0);}
   assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`overflow at ${width}`);
   console.log(`PASS ${width}px: current CMS records and responsive layout`);
  }
  assert.deepEqual(errors,[]);
 } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
