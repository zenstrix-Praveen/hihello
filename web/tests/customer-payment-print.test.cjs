const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const ts = require('typescript');

function load(globals = {}) {
  const file = path.join(__dirname, '../src/lib/customer-payment-print.ts');
  if (!fs.existsSync(file)) return {};
  const context = {exports:{}, ...globals};
  vm.runInNewContext(ts.transpileModule(fs.readFileSync(file,'utf8'), {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText,context);
  return context.exports;
}
const receipt = (provider='razorpay') => ({language:'en',branding:'Digital Card Platform',heading:'Payment Details',fields:[
  {label:'Customer Name',value:'Alex <script>alert(1)</script>'},
  {label:'Customer Email',value:'alex@example.test',direction:'ltr'},
  {label:'Payment Reference',value:'payment_'+ '0123456789'.repeat(30),direction:'ltr'},
  {label:'Plan',value:'Professional & Team'}, {label:'Amount',value:'INR 1,250.00'},
  {label:'Provider',value:provider}, {label:'Payment Method',value:'card'}, {label:'Status',value:'paid'},
  {label:'Provider Order ID',value:'order_'+ 'abcdefghij'.repeat(30),direction:'ltr'},
  {label:'Provider Payment ID',value:provider==='mock'?'—':'pay_1234567890',direction:'ltr'},
  {label:'Paid Date',value:'Oct 7, 2026'}, {label:'Created Date',value:'Oct 7, 2026'},
  {label:'Subscription',value:'active'}, {label:'Subscription Period',value:'Oct 7, 2026 – Oct 7, 2027'},
]});
test('receipt preserves every payment field and escapes customer input for mock and Razorpay',()=>{
  const {createPaymentReceiptDocument} = load();
  assert.equal(typeof createPaymentReceiptDocument,'function','isolated receipt renderer must exist');
  for(const provider of ['mock','razorpay']) {
    const data=receipt(provider), html=createPaymentReceiptDocument(data);
    for(const field of data.fields) {
      assert.ok(html.includes(field.label));
      if(!/[<&]/.test(field.value)) assert.ok(html.includes(field.value),field.label+' must not be shortened');
    }
    assert.ok(html.includes('Alex &lt;script&gt;alert(1)&lt;/script&gt;'));
    assert.ok(html.includes('Professional &amp; Team'));
    assert.ok(!html.includes('<script>'));
  }
});
test('receipt carries RTL language while identifiers retain LTR direction',()=>{
  const {createPaymentReceiptDocument} = load();
  assert.equal(typeof createPaymentReceiptDocument,'function');
  const html=createPaymentReceiptDocument({...receipt(),language:'ar',heading:'تفاصيل الدفع'});
  assert.match(html,/<html lang="ar" dir="rtl">/);
  assert.match(html,/<bdi dir="ltr">alex@example.test<\/bdi>/);
});

function browserFixture({height=600, failPrint=false}={}) {
  const events=new Map(), parentEvents=new Map(), frames=[], timers=new Map(); let timer=0, prints=0, focused=0, written='';
  let loaded, fontsReady;
  const doc={fonts:{ready:new Promise(resolve=>{fontsReady=resolve;})},body:{style:{}},getElementById:()=>({getBoundingClientRect:()=>({height,width:718})})};
  const child={document:doc,focus(){focused++;},print(){prints++;if(failPrint)throw Error('print unavailable');},addEventListener:(name,fn)=>events.set(name,fn),removeEventListener:(name)=>events.delete(name)};
  const frame={style:{},setAttribute(){},contentWindow:child,contentDocument:doc,addEventListener:(name,fn)=>{if(name==='load')loaded=fn;},removeEventListener(){},remove(){frames.splice(frames.indexOf(frame),1);},set srcdoc(value){written=value;}};
  const api=load({document:{createElement:()=>frame,body:{appendChild:node=>frames.push(node)}},window:{addEventListener:(name,fn)=>parentEvents.set(name,fn),removeEventListener:name=>parentEvents.delete(name)},setTimeout:(fn)=>{timers.set(++timer,fn);return timer;},clearTimeout:id=>timers.delete(id)});
  return {api,frames,doc,events,parentEvents,timers,load:()=>loaded(),fonts:()=>fontsReady(),printed:()=>prints,focused:()=>focused,html:()=>written};
}
const tick=()=>new Promise(resolve=>setImmediate(resolve));
test('prints only the loaded iframe, waits for fonts, fits overflow, and cleans up after dialog closes',async()=>{
  const f=browserFixture({height:1500});
  assert.equal(typeof f.api.printPaymentReceipt,'function','isolated print lifecycle must exist');
  const done=f.api.printPaymentReceipt(receipt());
  assert.equal(f.frames.length,1);
  assert.equal(f.printed(),0);
  f.load(); await tick(); assert.equal(f.printed(),0);
  f.fonts(); await tick();
  assert.equal(f.printed(),1);assert.equal(f.focused(),1);
  assert.ok(Number(f.doc.body.style.zoom)<1,'oversized content must be fitted without dropping fields');
  assert.equal(f.frames.length,1,'receipt stays alive while print dialog is open');
  f.events.get('afterprint')(); await done;
  assert.equal(f.frames.length,0);assert.equal(f.timers.size,0);assert.equal(f.parentEvents.size,0);
});
test('print failures release the temporary document',async()=>{
  const f=browserFixture({failPrint:true});
  assert.equal(typeof f.api.printPaymentReceipt,'function');
  const done=f.api.printPaymentReceipt(receipt());
  const rejected=assert.rejects(done,/print unavailable/);
  f.load();f.fonts();await rejected;
  assert.equal(f.frames.length,0);assert.equal(f.timers.size,0);
});
test('a receipt that fails to load times out without printing or leaking an iframe',async()=>{
  const f=browserFixture();
  const done=f.api.printPaymentReceipt(receipt());
  const rejected=assert.rejects(done,/Could not load payment receipt/);
  [...f.timers.values()][0]();await rejected;
  assert.equal(f.printed(),0);assert.equal(f.frames.length,0);assert.equal(f.parentEvents.size,0);
});
test('leaving the page releases a pending receipt and prevents delayed printing',async()=>{
  const f=browserFixture();
  const done=f.api.printPaymentReceipt(receipt());
  f.load();f.parentEvents.get('pagehide')();f.fonts();await done;await tick();
  assert.equal(f.printed(),0);assert.equal(f.frames.length,0);assert.equal(f.timers.size,0);
});
