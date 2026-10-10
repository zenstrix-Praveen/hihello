const test=require('node:test'), assert=require('node:assert/strict'), fs=require('node:fs'), ts=require('typescript'), React=require('react');
const {renderToStaticMarkup}=require('react-dom/server');
function load(file,deps){const code=ts.transpileModule(fs.readFileSync(`${__dirname}/../src/${file}`,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX,esModuleInterop:false}}).outputText;const mod={exports:{}};new Function('require','module','exports',code)(name=>deps[name]??(name==='@/lib/api-config'?{getApiBaseUrl:()=> 'http://localhost:8080/api/v1'}:name==='@/lib/customer-locale'?{useCustomerLocale:()=>({language:'en',t:value=>value,error:value=>value,setLanguage:()=>{}})}:name==='@/lib/features-page-content'?load('lib/features-page-content.ts',{}):name==='@/lib/customer-upgrade'?load('lib/customer-upgrade.ts',{}):name==='@/lib/global-pricing'?load('lib/global-pricing.ts',{}):name==='@/components/pricing/CurrencySelection'?{default:()=>null,CurrencyDropdown:()=>null,usePricingCurrency:()=>({currency:'',update(){}}),useCurrencyChoice:()=>({choice:{currency:'',country:''},update(){}})}:require(name)),mod,mod.exports);return mod.exports;}
test('unpublished CMS response hides content instead of restoring defaults',async()=>{let state,effect;const hook=load('components/public/useWebsiteSection.ts',{react:{useCallback:fn=>fn,useState:value=>[state=value,value=>{state=typeof value==='function'?value(state):value;}],useRef:value=>({current:value}),useEffect:fn=>{effect=fn;}}});const original=global.fetch;global.fetch=async()=>({ok:false,status:404});try{hook.useWebsiteSection('contact',{enabled:true,description:'Fallback'});effect();await new Promise(setImmediate);assert.equal(state.enabled,false);assert.equal(state.description,'Fallback');}finally{global.fetch=original;}});
test('public page fallback keeps content visible when its CMS endpoint is unavailable',async()=>{let state,effect;const defaults={enabled:true,page_title:'Choose a plan'};const hook=load('components/public/useWebsiteSection.ts',{react:{useCallback:fn=>fn,useState:value=>[state=value,value=>{state=typeof value==='function'?value(state):value;}],useRef:value=>({current:value}),useEffect:fn=>{effect=fn;}}});const original=global.fetch;global.fetch=async()=>({ok:false,status:404});try{const content=hook.useWebsiteSection('pricing',defaults,{fallbackOnUnavailable:true});assert.equal(content.enabled,true);effect();await new Promise(setImmediate);assert.deepEqual(state,defaults);}finally{global.fetch=original;}});
test('public CMS-backed pages opt into static content fallback; explicit disable flags remain honored',()=>{for(const file of ['app/page.tsx','app/features/page.tsx','app/templates/page.tsx','app/pricing/page.tsx','app/business/page.tsx','app/contact/page.tsx']){const source=fs.readFileSync(`${__dirname}/../src/${file}`,'utf8');assert.match(source,/fallbackOnUnavailable:\s*true/,`${file} should render its existing defaults if CMS content is missing`);}for(const file of ['app/features/page.tsx','app/templates/page.tsx','app/pricing/page.tsx','app/business/page.tsx','app/contact/page.tsx'])assert.match(fs.readFileSync(`${__dirname}/../src/${file}`,'utf8'),/enabled\s*===\s*false/,`${file} should still honor an explicit CMS disable flag`);});
function pricing(plans,useSection=()=>({enabled:true})){let index=0;const planUtils=load('lib/pricing-plans.ts',{});const PlanPricingCard=load('components/pricing/PlanPricingCard.tsx',{'next/link':{default:({children,href})=>React.createElement('a',{href},children)},'@/lib/pricing-plans':planUtils}).default;const Component=load('app/pricing/page.tsx',{react:{...React,useRef:value=>({current:value}),useEffect:()=>{},useState:initial=>[index++===0?plans:initial===true?false:initial,()=>{}],useMemo:fn=>fn()},'next/link':{default:({children,href})=>React.createElement('a',{href},children)},'next/navigation':{useRouter:()=>({push:()=>{}})},'@/components/public/PublicHeader':{default:()=>null},'@/components/public/PublicFooter':{default:()=>null},'@/lib/user-api':{getUserToken:()=>null},'@/components/public/useWebsiteSection':{useWebsiteSection:useSection},'@/components/pricing/PlanPricingCard':{default:PlanPricingCard},'@/lib/pricing-plans':planUtils}).default;return renderToStaticMarkup(React.createElement(Component));}
function pricingCard(html,name){const at=html.indexOf(name);const start=html.lastIndexOf('<article',at);const end=html.indexOf('</article>',at)+10;return html.slice(start,end);}
test('Pricing destination renders its fallback page content when the CMS section is missing',()=>{let options;const html=pricing([],(_section,defaults,receivedOptions)=>{options=receivedOptions;return {...defaults};});assert.equal(options.fallbackOnUnavailable,true);assert.match(html,/<main/);assert.match(html,/Choose the plan that fits you/);});
test('pricing renders saved names and never resurrects deleted tiers',()=>{const html=pricing([{id:91,name:'NFC Team',description:'Saved CMS description',price:12,currency:'USD',billing_period:'monthly',pricing_type:'fixed',feature_limits:{features:['Sharing']},is_featured:false,trial_days:0}]);assert.match(html,/NFC Team/);assert.match(html,/Saved CMS description/);assert.doesNotMatch(html,/>Personal</);assert.doesNotMatch(pricing([]),/>Personal<|>Enterprise</);assert.match(pricing([]),/No .* plans are available yet/);});
test('Pricing renders zero-price plans first and preserves configured order for paid plans',()=>{
 const html=pricing([
  {id:111,name:'Professional',description:'',price:25,currency:'SAR',billing_period:'monthly',pricing_type:'fixed',feature_limits:{features:[]},trial_days:0},
  {id:112,name:'Business',description:'',price:100,currency:'SAR',billing_period:'monthly',pricing_type:'fixed',feature_limits:{features:[]},trial_days:0},
  {id:113,name:'Normal Users',description:'',price:0,currency:'SAR',billing_period:'lifetime',pricing_type:'free',feature_limits:{features:[]},trial_days:0},
 ]);
 assert.ok(html.indexOf('Normal Users')<html.indexOf('Professional'));
 assert.ok(html.indexOf('Professional')<html.indexOf('Business'));
});
test('pricing cards put saved price and billing period below benefits, before the action',()=>{
 const html=pricing([
  {id:92,name:'Regional',description:'Regional plan details',price:120,currency:'SAR',billing_period:'monthly',pricing_type:'fixed',feature_limits:{features:['Sharing','Analytics']},is_featured:false,trial_days:0},
  {id:93,name:'Starter',description:'Starter details',price:0,currency:'USD',billing_period:'monthly',pricing_type:'free',feature_limits:{features:['QR Sharing']},is_featured:false,trial_days:0},
 ]);
 const card=pricingCard(html,'Regional');
 assert.ok(card.indexOf('Regional')<card.indexOf('included'));
 assert.ok(card.indexOf('included')<card.indexOf('Sharing'));
 assert.ok(card.indexOf('Sharing')<card.indexOf('SAR'));
 assert.ok(card.indexOf('SAR')<card.indexOf('per month'));
 assert.ok(card.indexOf('per month')<card.indexOf('Upgrade Plan'));
 assert.match(html,/Free/);
 assert.match(html,/Forever/);
 assert.equal((card.match(/text-3xl font-black tracking-tight text-slate-950/g)||[]).length,1);
 assert.match(html,/grid auto-rows-fr gap-5 sm:grid-cols-2 lg:grid-cols-3/);
 assert.match(html,/data-pricing-bottom/);
 assert.match(html,/mt-auto pt-7/);
});
test('Pricing only shows Most Popular when a plan has that setting enabled',()=>{
 const base={id:94,name:'Featured only',description:'',price:15,currency:'USD',billing_period:'monthly',pricing_type:'fixed',feature_limits:{features:[]},is_featured:true,trial_days:0};
 assert.doesNotMatch(pricing([base]),/>Most Popular</);
 assert.match(pricing([{...base,is_most_popular:true}]),/>Most Popular</);
});
test('monthly and yearly plans remain separate and savings require configured matching prices',()=>{
 const {plansForBillingPeriod,maximumAnnualSavings,prioritizeFreePlans}=load('lib/pricing-plans.ts',{});
 const monthly={id:101,code:'pro-monthly',name:'Pro',description:'',price:10,currency:'SAR',billing_period:'monthly',trial_days:0};
 const yearly={...monthly,id:102,code:'pro-yearly',price:96,billing_period:'yearly'};
 const free={...monthly,id:103,code:'free',price:0,billing_period:'lifetime'};
 assert.deepEqual(plansForBillingPeriod([monthly,yearly,free],'monthly').map(plan=>plan.id),[101,103]);
 assert.deepEqual(plansForBillingPeriod([monthly,yearly,free],'yearly').map(plan=>plan.id),[102,103]);
 assert.equal(maximumAnnualSavings([monthly,yearly]),20);
 assert.equal(maximumAnnualSavings([monthly,{...yearly,currency:'USD'}]),0);
 assert.equal(maximumAnnualSavings([monthly]),0);
 assert.deepEqual(prioritizeFreePlans([monthly,yearly,free]).map(plan=>plan.id),[103,101,102]);
});
test('pricing CTA labels use saved price while retaining current-plan states and destinations',()=>{
 const html=pricing([
  {id:95,name:'Paid saved tier',description:'',price:25,currency:'SAR',billing_period:'monthly',pricing_type:'fixed',feature_limits:{features:[]},is_featured:false,trial_days:0},
  {id:96,name:'Free saved tier',description:'',price:0,currency:'USD',billing_period:'monthly',pricing_type:'free',feature_limits:{features:[]},is_featured:false,trial_days:0},
  {id:97,name:'Paid linked tier',description:'',price:100,currency:'SAR',billing_period:'monthly',pricing_type:'fixed',cta_url:'/custom-checkout',feature_limits:{features:[]},is_featured:false,trial_days:0},
 ]);
 const paid=pricingCard(html,'Paid saved tier');
 const free=pricingCard(html,'Free saved tier');
 const linked=pricingCard(html,'Paid linked tier');
 assert.match(paid,/>Upgrade Plan</);
 assert.match(free,/>Get Started</);
 assert.doesNotMatch(linked,/href="\/custom-checkout"/,'paid purchases must use the authenticated billing flow');
 assert.match(linked,/>Upgrade Plan</);
 assert.match(paid,/bg-gradient-to-r from-violet-600 to-purple-600/);
 assert.match(fs.readFileSync(`${__dirname}/../src/app/pricing/page.tsx`,'utf8'),/onAction=\{\(\) => choose\(plan\)\}/);
});

function publicPage(file,cms,extra={}) {
 const Page=load(file,{react:{...React,useRef:value=>({current:value}),useEffect:()=>{},useState:value=>[value,()=>{}],useRef:value=>({current:value})},'next/link':{default:({children,href})=>React.createElement('a',{href},children)},'next/navigation':{useRouter:()=>({push(){}})},'@/components/public/PublicHeader':{default:()=>null},'@/components/public/PublicFooter':{default:()=>null},'@/lib/user-api':{getUserToken:()=>null},'@/components/public/useWebsiteSection':{useWebsiteSection:()=>cms},...extra}).default;
 return renderToStaticMarkup(React.createElement(Page));
}
test('clearing business benefits removes them instead of restoring static records',()=>{
 const html=publicPage('app/business/page.tsx',{enabled:true,benefits:[]});
 assert.doesNotMatch(html,/Brand Control|Secure Administration/);
});
test('contact displays saved business hours',()=>{
 assert.match(publicPage('app/contact/page.tsx',{enabled:true,business_hours:'Monday 09:00–17:00'}),/Monday 09:00–17:00/);
});
test('contact page includes localized about and FAQ sections before the validated message form',()=>{
 const arabic=load('lib/customer-public-messages.ts',{}).customerPublicMessages;
 const html=publicPage('app/contact/page.tsx',{enabled:true,contact_page_heading:'اتصل بنا',description:'تواصل معنا',form_heading:'أرسل لنا رسالة'}, {'@/lib/customer-locale':{useCustomerLocale:()=>({t:value=>arabic[value]??value,error:value=>String(value)})}});
 assert.match(html,/id="about-us"/);assert.match(html,/id="faq"/);assert.match(html,/id="send-message"/);
 assert.match(html,/تعرف علينا/);assert.match(html,/ما بطاقة الأعمال الرقمية؟/);assert.match(html,/كيف أتواصل مع دعم العملاء؟/);
 assert.match(html,/<details/);assert.match(html,/<summary/);assert.match(html,/required/);
 assert.match(html,/sm:grid-cols-3/);assert.match(html,/sm:grid-cols-2/);assert.match(html,/max-w-5xl/);
 for(const key of ['Know About Us','Send us a message','Send a message and our team will follow up.','Your message has been received.','How do I create a digital card?','Where can I review a payment?','How can I manage my account information?','How can I contact customer support?'])assert.ok(arabic[key],`Missing Arabic text for ${key}`);
});
test('contact form keeps its POST submission and reports the CMS success message',async()=>{
 let hookIndex=0;const states=[];const calls=[];
 const hooks={...React,useState(initial){const i=hookIndex++;if(!(i in states))states[i]=initial;return [states[i],value=>{states[i]=typeof value==='function'?value(states[i]):value;}];}};
 const cms={enabled:true,contact_page_heading:'Contact us',description:'Send a message',form_heading:'Send us a message',success_message:'Message received'};
 const Page=load('app/contact/page.tsx',{react:hooks,'@/components/public/PublicHeader':{default:()=>null},'@/components/public/PublicFooter':{default:()=>null},'@/components/public/useWebsiteSection':{useWebsiteSection:()=>cms},'@/lib/user-api':{apiRequest:async(path,options)=>{calls.push({path,options});return {}; }},'@/lib/customer-locale':{useCustomerLocale:()=>({t:value=>value,error:value=>String(value)})}}).default;
 const render=()=>{hookIndex=0;return Page();};
 let tree=render();const form=find(tree,node=>node.type==='form');
 const fields=[];function collect(node){if(!node||typeof node!=='object')return;if(node.type==='input'||node.type==='textarea')fields.push(node);for(const child of [node.props?.children].flat(Infinity))collect(child);}
 collect(form);const values=['Alex Example','alex@example.com','+1 555 0100','Product question','Please tell me more.'];
 fields.forEach((field,index)=>field.props.onChange({target:{value:values[index]}}));
 tree=render();const activeForm=find(tree,node=>node.type==='form');await activeForm.props.onSubmit({preventDefault(){}});
 assert.equal(calls.length,1);assert.equal(calls[0].path,'/website/contact');assert.equal(calls[0].options.method,'POST');
 assert.deepEqual(JSON.parse(calls[0].options.body),{name:values[0],email:values[1],phone:values[2],subject:values[3],message:values[4]});
 assert.match(textOf(render()),/Message received/);
});
for(const file of ['business','contact'])test(`unpublished ${file} hides its content`,()=>{assert.equal(publicPage(`app/${file}/page.tsx`,{enabled:false,description:'Private content'}),'');});
test('features use one CMS request and render only its current records',()=>{
 let calls=0;const cards=load('components/public/CMSFeatureCards.tsx',{});
 const html=publicPage('app/features/page.tsx',{}, {'@/components/public/CMSFeatureCards':cards,'@/components/public/useWebsiteSection':{useWebsiteSection:()=>{calls++;return {enabled:true,items:[{id:1,title:'NFC CMS record',description:'Current copy',is_enabled:true},{id:2,title:'Unpublished record',is_enabled:false}]};}}});
 assert.equal(calls,1);assert.match(html,/NFC CMS record/);assert.doesNotMatch(html,/Unpublished record|Build a card that feels like you/);
});
test('CMS sections remain hidden while visibility is loading or fetch fails',async()=>{
 let state,effect;const hook=load('components/public/useWebsiteSection.ts',{react:{useCallback:fn=>fn,useState:value=>[state=value,value=>{state=typeof value==='function'?value(state):value;}],useRef:value=>({current:value}),useEffect:fn=>{effect=fn;}}});const original=global.fetch;global.fetch=async()=>{throw new Error('offline')};try{hook.useWebsiteSection('contact',{enabled:true});assert.equal(state.enabled,false);effect();await new Promise(setImmediate);assert.equal(state.enabled,false);}finally{global.fetch=original;}
});
function adminView(file,api) {
 const states=[],slots=[];let index=0;const effects=[];
 const memo=(fn,deps)=>{const i=index++;if(!slots[i]||deps.some((v,n)=>v!==slots[i].deps[n]))slots[i]={value:fn(),deps};return slots[i].value;};
 const hooks={useRef:value=>memo(()=>({current:value}),[]),useState:initial=>{const i=index++;if(!(i in states))states[i]=initial;return [states[i],value=>{states[i]=typeof value==='function'?value(states[i]):value}]},useCallback:(fn,deps)=>memo(()=>fn,deps),useEffect:(fn,deps)=>memo(()=>{effects.push(fn)},deps)};
 const Page=load(file,{react:hooks,'@/lib/admin-api':{adminApi:api},'@/components/admin/WebsitePageContentForm':{default:()=>null}}).default;
 return {render:()=>{index=0;return Page()},effects,states};
}
function find(node,predicate){if(!node||typeof node!=='object')return;if(predicate(node))return node;for(const child of [node.props?.children].flat(Infinity)){const result=find(child,predicate);if(result)return result;}}
function textOf(node){if(node==null)return '';if(typeof node!=='object')return String(node);return [node.props?.children].flat(Infinity).map(textOf).join('');}
test('Admin Features CRUD reloads saved records without a render fetch loop',async t=>{
 const savedWindow=global.window;global.window={confirm:()=>true};t.after(()=>{global.window=savedWindow});let records=[],calls=0;
 const api=async(path,options={})=>{calls++;const method=options.method||'GET';if(path.includes('/sections/'))return {enabled:true,page_heading:'Saved heading'};if(method==='GET')return {items:structuredClone(records)};if(method==='POST'){records.push({...JSON.parse(options.body),id:7});return records[0];}if(method==='PUT'){records=[{...JSON.parse(options.body),id:7}];return records[0];}if(method==='DELETE'){records=[];return {message:'Deleted'};}};
 const fresh=async()=>{const view=adminView('app/admin/website/features/page.tsx',api);assert.match(textOf(view.render()),/Loading saved features/);await view.effects[0]();await new Promise(setImmediate);view.render();return view;};
 let view=await fresh();assert.match(textOf(view.render()),/No items found/);const before=calls;view.render();view.render();assert.equal(view.effects.length,1);assert.equal(calls,before);
 const title=find(view.render(),n=>n.type==='input'&&n.props.required);title.props.onChange({target:{value:'NFC UI item'}});const form=find(view.render(),n=>n.type==='form'&&textOf(n).includes('Add Feature'));await form.props.onSubmit({preventDefault(){}});assert.match(textOf(view.render()),/NFC UI item/);
 view=await fresh();assert.match(textOf(view.render()),/NFC UI item/);find(view.render(),n=>n.type==='button'&&textOf(n)==='Edit').props.onClick();find(view.render(),n=>n.type==='input'&&n.props.required).props.onChange({target:{value:'Edited NFC item'}});await find(view.render(),n=>n.type==='form'&&textOf(n).includes('Edit Feature')).props.onSubmit({preventDefault(){}});
 view=await fresh();assert.match(textOf(view.render()),/Edited NFC item/);find(view.render(),n=>n.type==='button'&&textOf(n)==='Disable').props.onClick();await new Promise(setImmediate);assert.equal(records[0].is_enabled,false);
 find(view.render(),n=>n.type==='button'&&textOf(n)==='Delete').props.onClick();await new Promise(setImmediate);assert.doesNotMatch(textOf(view.render()),/Edited NFC item/);view=await fresh();assert.match(textOf(view.render()),/No items found/);
});
test('Admin CMS lists expose load errors',async()=>{
 for(const name of ['features','pricing']){const view=adminView(`app/admin/website/${name}/page.tsx`,async()=>{throw new Error('CMS unavailable')});view.render();await view.effects[0]();await new Promise(setImmediate);assert.match(textOf(find(view.render(),n=>n.props?.role==='alert')),/CMS unavailable/);}
});
test('business displays the saved sales contact email',()=>{
 assert.match(publicPage('app/business/page.tsx',{enabled:true,benefits:[],contact_sales_email:'cms-sales@example.com'}),/href="mailto:cms-sales@example.com"/);
});
