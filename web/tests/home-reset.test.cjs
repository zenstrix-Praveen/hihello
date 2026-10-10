const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),ts=require('typescript'),React=require('react');
const {renderToStaticMarkup}=require('react-dom/server');
function load(file,deps){const code=ts.transpileModule(fs.readFileSync(`${__dirname}/../src/${file}`,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,jsx:ts.JsxEmit.ReactJSX,esModuleInterop:false}}).outputText;const mod={exports:{}};new Function('require','module','exports',code)(name=>deps[name]??(name==='@/lib/customer-locale'?{useCustomerLocale:()=>({language:'en',t:value=>value,error:value=>value})}:require(name)),mod,mod.exports);return mod.exports.default;}
const empty={hero_title:'',hero_subtitle:'',primary_button_text:'',primary_button_url:'',secondary_button_text:'',secondary_button_url:'',hero_image_url:''};
function view(api){const states=[],effects=[],deps=[];let index=0;const Page=load('app/admin/website/home/page.tsx',{react:{useState:value=>{const i=index++;if(!(i in states))states[i]=value;return [states[i],value=>{states[i]=typeof value==='function'?value(states[i]):value}]},useEffect:(fn,values)=>{const i=index++;if(!deps[i]||values.some((value,n)=>value!==deps[i][n])){deps[i]=values;effects.push(fn)}}},'@/lib/admin-locale':{useAdminLocale:()=>({t:x=>x})},'@/lib/admin-api':{adminApi:api}});return {render:()=>{index=0;return Page()},effects};}
function text(node){if(node==null)return '';if(typeof node!=='object')return String(node);return [node.props?.children].flat(Infinity).map(text).join('');}
function find(node,predicate){if(!node||typeof node!=='object')return;if(predicate(node))return node;for(const child of [node.props?.children].flat(Infinity)){const found=find(child,predicate);if(found)return found;}}
const button=v=>find(v.render(),n=>n.type==='button'&&text(n)==='Reset Home Page');
const field=(v,name)=>find(v.render(),n=>n.props?.name===name);
const flush=()=>new Promise(setImmediate);
test('Home reset confirms, refetches persisted values, survives refresh and permits saving again',async t=>{
 const previous=global.window;t.after(()=>{global.window=previous});let confirmed=false,stored={...empty,hero_title:'Temporary CMS home',hero_subtitle:'Saved copy'},calls=[];
 global.window={confirm:message=>{assert.match(message,/Custom Home content will be removed/);return confirmed}};
 const api=async(path,options={})=>{calls.push([path,options.method||'GET']);if(options.method==='POST'){stored={...empty};return {success:true,data:{...stored}}}if(options.method==='PUT')stored=JSON.parse(options.body);return {...stored}};
 const fresh=async()=>{const v=view(api);v.render();v.effects[0]();await flush();return v};
 let v=await fresh();assert.equal(field(v,'hero_title').props.value,'Temporary CMS home');assert(button(v),'Reset action is visible');await button(v).props.onClick();assert.equal(calls.length,1);
 confirmed=true;await button(v).props.onClick();assert.deepEqual(calls.slice(-2),[['/admin/website/home/reset','POST'],['/admin/website/home','GET']]);assert.equal(field(v,'hero_title').props.value,'');assert.match(text(v.render()),/Home Page content reset successfully/);
 v=await fresh();assert.equal(field(v,'hero_title').props.value,'');const before=calls.length;v.render();v.render();assert.equal(v.effects.length,1);assert.equal(calls.length,before);
 field(v,'hero_title').props.onChange({target:{name:'hero_title',value:'New Home Title'}});field(v,'hero_subtitle').props.onChange({target:{name:'hero_subtitle',value:'New Home Subtitle'}});await find(v.render(),n=>n.type==='form').props.onSubmit({preventDefault(){}});
 v=await fresh();assert.equal(field(v,'hero_title').props.value,'New Home Title');assert.equal(field(v,'hero_subtitle').props.value,'New Home Subtitle');
});
test('reset errors are visible and leave existing values intact',async t=>{
 const previous=global.window;global.window={confirm:()=>true};t.after(()=>{global.window=previous});const v=view(async(path,options={})=>{if(options.method==='POST')throw new Error('Could not reset Home Page content.');return {...empty,hero_title:'Keep this title'}});v.render();v.effects[0]();await flush();assert(button(v));await button(v).props.onClick();assert.match(text(find(v.render(),n=>n.props?.role==='alert')),/Could not reset/);assert.equal(field(v,'hero_title').props.value,'Keep this title');
});
test('public Home renders existing fallbacks for reset fields and new content after save',()=>{
 function render(cms){const Page=load('app/page.tsx',{react:{useState:value=>[value,()=>{}],useEffect:()=>{}},'next/link':{default:({href,children})=>React.createElement('a',{href},children)},'@/components/public/PublicHeader':{default:()=>null},'@/components/public/PublicFooter':{default:()=>null},'@/components/public/HomeProductStory':{default:()=>null},'@/components/public/useWebsiteSection':{useWebsiteSection:()=>cms}});return renderToStaticMarkup(React.createElement(Page));}
 const reset=render(empty);assert.match(reset,/Make every professional connection count/);assert.match(reset,/href="\/register"/);assert.match(reset,/href="\/templates"/);assert.doesNotMatch(reset,/Temporary CMS home|Saved copy/);
 assert.match(render({...empty,hero_title:'New Home Title',hero_subtitle:'New Home Subtitle'}),/New Home Title/);
});
