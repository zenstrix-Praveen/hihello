const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const ts = require('typescript');
function load(file, deps={}) {
  const filename=path.join(__dirname,'../src',file);
  if(!fs.existsSync(filename))return {};
  const module={exports:{}};
  const code=ts.transpileModule(fs.readFileSync(filename,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020,jsx:ts.JsxEmit.ReactJSX}}).outputText;
  new Function('module','exports','require',code)(module,module.exports,id=>deps[id]??require(id));
  return module.exports;
}
test('currency search matches code and translated name without altering ISO values',()=>{
  const {searchPlanCurrencies}=load('lib/plan-currencies.ts');
  assert.equal(typeof searchPlanCurrencies,'function');
  const currencies=[['USD','US Dollar'],['INR','Indian Rupee'],['AED','UAE Dirham'],['SAR','Saudi Riyal'],['QAR','Qatari Riyal'],['KWD','Kuwaiti Dinar'],['BHD','Bahraini Dinar'],['OMR','Omani Rial'],['EGP','Egyptian Pound'],['JOD','Jordanian Dinar'],['EUR','Euro'],['GBP','British Pound']].map(([code,name])=>({code,name,symbol:code,exponent:2}));
  const t=key=>({'UAE Dirham':'درهم إماراتي','Saudi Riyal':'ريال سعودي'}[key]||key);
  assert.deepEqual(searchPlanCurrencies('dirham',t,currencies).map(item=>item.code),['AED']);
  assert.deepEqual(searchPlanCurrencies('سعودي',t,currencies).map(item=>item.code),['SAR']);
  assert.deepEqual(searchPlanCurrencies(' aed ',t,currencies).map(item=>item.code),['AED']);
  assert.equal(searchPlanCurrencies('nonsense',t,currencies).length,0);
});
function find(node,predicate) {
  if(!node||typeof node!=='object')return;
  if(predicate(node))return node;
  for(const child of [node.props?.children].flat(Infinity)){const result=find(child,predicate);if(result)return result;}
}
function harness(initial='USD') {
  const states=[];let index=0,value=initial,language='en';const changes=[];
  const messages=load('lib/plan-currency-messages.ts').currencyArabicMessages||{};
  const currencies=[['USD','US Dollar'],['INR','Indian Rupee'],['AED','UAE Dirham'],['SAR','Saudi Riyal'],['QAR','Qatari Riyal'],['KWD','Kuwaiti Dinar'],['BHD','Bahraini Dinar'],['OMR','Omani Rial'],['EGP','Egyptian Pound'],['JOD','Jordanian Dinar'],['EUR','Euro'],['GBP','British Pound']].map(([code,name])=>({code,name,symbol:code,exponent:2}));
  const hooks={useId:()=> 'currency',useRef:()=>({current:null}),useEffect:()=>{},useState:init=>{const i=index++;if(!(i in states))states[i]=i===3?currencies:init;return [states[i],next=>{states[i]=typeof next==='function'?next(states[i]):next;}];}};
  const Component=load('components/admin/PlanCurrencySelect.tsx',{react:hooks,'@/lib/admin-locale':{useAdminLocale:()=>({language,t:key=>language==='ar'?(messages[key]||key):key})},'@/lib/plan-currencies':load('lib/plan-currencies.ts'),'@/lib/api-config':{getApiBaseUrl:()=>''},'@/lib/admin-api':{adminApi:async()=>({})}}).default;
  assert.equal(typeof Component,'function','searchable currency selector must exist');
  return {changes,language:lang=>{language=lang;},render:()=>{index=0;return Component({value,existingCurrency:initial,onChange:code=>{changes.push(code);value=code;}});}};
}
test('picker commits only the selected ISO code, supports keyboard, and updates Arabic labels',()=>{
  const h=harness();let tree=h.render();let input=find(tree,n=>n.props?.role==='combobox');
  assert.match(input.props.value,/USD.*US Dollar/);
  input.props.onFocus();input.props.onChange({target:{value:'dirham'}});
  tree=h.render();input=find(tree,n=>n.props?.role==='combobox');
  input.props.onKeyDown({key:'Enter',preventDefault(){},stopPropagation(){}});
  assert.deepEqual(h.changes,['AED']);
  h.language('ar');tree=h.render();input=find(tree,n=>n.props?.role==='combobox');
  assert.match(input.props.value,/AED.*درهم إماراتي/);
  input.props.onFocus();input.props.onChange({target:{value:'INVALID'}});
  input=find(h.render(),n=>n.props?.role==='combobox');input.props.onKeyDown({key:'Enter',preventDefault(){},stopPropagation(){}});
  assert.deepEqual(h.changes,['AED']);
});
test('editing displays the existing selection without emitting a currency or price change',()=>{
  for(const code of ['USD','AED','SAR','CAD']) {
    const h=harness(code),tree=h.render();
    assert.ok(find(tree,n=>n.props?.role==='combobox').props.value.startsWith(code));
    assert.deepEqual(h.changes,[]);
  }
});
