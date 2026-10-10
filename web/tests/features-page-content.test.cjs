const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),ts=require('typescript'),React=require('react');
const {renderToStaticMarkup}=require('react-dom/server');
function load(file,deps={}){const code=ts.transpileModule(fs.readFileSync(`${__dirname}/../src/${file}`,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX,esModuleInterop:false}}).outputText;const mod={exports:{}};new Function('require','module','exports',code)(name=>deps[name]??(name==='@/lib/customer-locale'?{useCustomerLocale:()=>({language:'en',t:value=>value,error:value=>value})}:name==='@/lib/features-page-content'?load('lib/features-page-content.ts'):require(name)),mod,mod.exports);return mod.exports;}
const {featuresPageDefaults:defaults}=load('lib/features-page-content.ts');
function view(api){const slots=[],effects=[];let i=0;const memo=(fn,deps)=>{const n=i++;if(!slots[n]||deps.some((v,k)=>v!==slots[n].deps[k]))slots[n]={value:fn(),deps};return slots[n].value};const Page=load('app/admin/website/features/page.tsx',{react:{useState:value=>{const n=i++;if(!(n in slots))slots[n]=typeof value==='function'?value():value;return [slots[n],v=>slots[n]=typeof v==='function'?v(slots[n]):v]},useRef:value=>memo(()=>({current:value}),[]),useCallback:(fn,deps)=>memo(()=>fn,deps),useEffect:(fn,deps)=>memo(()=>effects.push(fn),deps)},'@/lib/admin-api':{adminApi:api}}).default;return {render(){i=0;return Page()},effects};}
function text(n){return n==null?'':typeof n!=='object'?String(n):[n.props?.children].flat(Infinity).map(text).join('')}
function find(n,p){if(!n||typeof n!=='object')return;if(p(n))return n;for(const c of [n.props?.children].flat(Infinity)){const r=find(c,p);if(r)return r}}
const button=(v,label)=>find((label==='Delete'||label==='Reset'||label==='Cancel')?find(v.render(),n=>n.props?.role==='dialog'):v.render(),n=>n.type==='button'&&text(n)===label);
const field=(v,name)=>find(v.render(),n=>n.props?.name===name);
const flush=()=>new Promise(setImmediate);
test('shared defaults match backend reset copy',()=>{
 const backend=fs.readFileSync(`${__dirname}/../../backend/internal/website/features_page_content.go`,'utf8');
 assert(backend.includes(JSON.stringify(defaults.page_heading)));
 assert(backend.includes(JSON.stringify(defaults.page_subtitle)));
 assert.equal(defaults.enabled,true);
});
async function fresh(api){const v=view(api);v.render();v.effects[0]();await flush();return v}
function store(){let saved={...defaults,page_heading:'Custom hihello',page_subtitle:'Custom subtitle'},calls=[];const items=[{id:7,title:'QR Sharing',is_enabled:true,sort_order:3}];return {calls,items,api:async(path,options={})=>{const method=options.method||'GET';calls.push([path,method]);if(path==='/admin/website/features')return {items};if(method==='PUT')saved=JSON.parse(options.body);if(method==='POST')saved={...defaults};if(method==='DELETE')saved={};return method==='GET'?{...saved}:{success:true,content:{...saved},data:{...saved}}}}}
for(const action of ['Reset','Delete'])test(`${action} confirms, persists, preserves cards and permits save again`,async()=>{const s=store();let v=await fresh(s.api);assert.equal(field(v,'page_heading').props.value,'Custom hihello');button(v,`${action} Page Content`).props.onClick();assert(find(v.render(),n=>n.props?.role==='dialog'));button(v,'Cancel').props.onClick();assert.equal(s.calls.length,2);button(v,`${action} Page Content`).props.onClick();await button(v,action).props.onClick();assert.equal(field(v,'page_heading').props.value,defaults.page_heading);assert.equal(field(v,'page_subtitle').props.value,defaults.page_subtitle);assert.equal(field(v,'enabled').props.checked,true);assert.match(text(v.render()),/QR Sharing/);assert.equal(s.calls.at(-1)[1],'GET');v=await fresh(s.api);assert.equal(field(v,'page_heading').props.value,defaults.page_heading);field(v,'page_heading').props.onChange({target:{value:'Professional Digital Card Features'}});field(v,'page_subtitle').props.onChange({target:{value:'New subtitle'}});await find(v.render(),n=>n.type==='form').props.onSubmit({preventDefault(){}});v=await fresh(s.api);assert.equal(field(v,'page_heading').props.value,'Professional Digital Card Features');assert.equal(field(v,'page_subtitle').props.value,'New subtitle');assert(s.calls.filter(([p,m])=>m!=='GET').every(([p])=>p.startsWith('/admin/website/sections/features')))});
for(const action of ['Reset','Delete'])test(`${action} failure keeps form; rapid confirmation sends one request`,async()=>{const s=store();let reject,calls=0;const v=await fresh((p,o={})=>o.method?(calls++,new Promise((_,r)=>reject=r)):s.api(p,o));button(v,`${action} Page Content`).props.onClick();const confirm=button(v,action);const pending=confirm.props.onClick();confirm.props.onClick();assert.equal(calls,1);for(const label of [action==='Reset'?'Resetting…':'Reset Page Content',action==='Delete'?'Deleting…':'Delete Page Content','Save Page Content'])assert(button(v,label).props.disabled);assert(field(v,'page_heading').props.disabled);reject(Error('internal'));await pending;assert.equal(field(v,'page_heading').props.value,'Custom hihello');assert.match(text(v.render()),new RegExp(`Could not ${action.toLowerCase()} Features page content\\.`))});
test('Save prevents same-tick duplicate submission and reloads persisted response',async()=>{const s=store();let resolve,calls=0;const v=await fresh((p,o={})=>o.method?(calls++,new Promise(r=>resolve=r)):s.api(p,o));const form=find(v.render(),n=>n.type==='form');const pending=form.props.onSubmit({preventDefault(){}});form.props.onSubmit({preventDefault(){}});assert.equal(calls,1);resolve({content:defaults});await pending;assert.equal(field(v,'page_heading').props.value,'Custom hihello')});
test('public Features uses defaults after delete and retains real CMS cards',()=>{const Cards=load('components/public/CMSFeatureCards.tsx').default;const Page=load('app/features/page.tsx',{'next/link':{default:({children,href})=>React.createElement('a',{href},children)},'@/components/public/PublicHeader':{default:()=>null},'@/components/public/PublicFooter':{default:()=>null},'@/components/public/CMSFeatureCards':{default:Cards},'@/components/public/useWebsiteSection':{useWebsiteSection:(_,fallback)=>({...fallback,items:[{id:7,title:'QR Sharing',is_enabled:true,sort_order:3}]})}}).default;const html=renderToStaticMarkup(React.createElement(Page));assert.match(html,/Everything Your Digital Card Needs/);assert.match(html,/QR Sharing/);assert.doesNotMatch(html,/hihello/)});

for(const action of ['Reset','Delete'])test(`${action} success followed by reload failure cannot resurrect custom values`,async()=>{
 const s=store();let mutated=false;
 const v=await fresh(async(p,o={})=>{
  if(mutated&&!o.method)throw Error('read failed');
  const result=await s.api(p,o);if(o.method)mutated=true;return result;
 });
 button(v,`${action} Page Content`).props.onClick();await button(v,action).props.onClick();
 assert.equal(field(v,'page_heading').props.value,defaults.page_heading);
 assert.equal(field(v,'page_subtitle').props.value,defaults.page_subtitle);
 assert.match(text(v.render()),/was updated, but could not be reloaded/);
 assert(!find(v.render(),n=>n.props?.role==='status'&&/deleted|reset to defaults/.test(text(n))));
});

test('late card-list reload cannot overwrite a completed page-content deletion',async()=>{
 const s=store();let reloadResolve,mutationResolve,reads=0;
 const v=await fresh(async(p,o={})=>{
  if(p.endsWith('/sections/features')&&!o.method&&++reads===2)return new Promise(r=>reloadResolve=r);
  if(o.method==='DELETE') {await new Promise(r=>mutationResolve=r);}
  return s.api(p,o);
 });
 button(v,'Delete Page Content').props.onClick();const pending=button(v,'Delete').props.onClick();
 // A concurrent card status action invokes the existing shared list loader.
 button(v,'Disable').props.onClick();await flush();
 mutationResolve();await pending;
 reloadResolve({page_heading:'Stale heading',page_subtitle:'Stale subtitle',enabled:false});await flush();
 assert.equal(field(v,'page_heading').props.value,defaults.page_heading);
 assert.equal(field(v,'enabled').props.checked,true);
});
