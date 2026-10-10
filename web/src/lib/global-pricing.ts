import type { PublicPlan } from "./pricing-plans";
export type PriceRule={currency:string;country:string;billing_period:string;amount:string;tax_bps:number;decimal_precision?:number;rounding?:string};
export type PriceConfig={prices:PriceRule[];providers:string[]};
export type QuotedPlan=PublicPlan & {subtotal?:string;tax?:string;quote_total?:string};
function exponent(currency:string){return new Intl.NumberFormat("en",{style:"currency",currency}).resolvedOptions().maximumFractionDigits ?? 2;}
function minor(amount:string,exp:number){
 if(!/^(0|[1-9][0-9]*)(\.[0-9]+)?$/.test(amount))throw new Error("Invalid price");
 const [whole,fraction=""]=amount.split(".");
 if(fraction.slice(exp).replace(/0/g,""))throw new Error("Invalid currency precision");
 return BigInt(whole+fraction.slice(0,exp).padEnd(exp,"0"));
}
function major(value:bigint,exp:number){const digits=value.toString().padStart(exp+1,"0");return exp?digits.slice(0,-exp)+"."+digits.slice(-exp):digits;}
/** Exact display calculations; the backend independently verifies the quote. */
export function quotePlan(plan:PublicPlan,currency:string,country:string,period:string):QuotedPlan|null {
 const code=(currency||plan.currency).toUpperCase(),exp=exponent(code);
 const rules=plan.billing_config?.prices||[];
 let amount=code===plan.currency.toUpperCase()&&period===plan.billing_period?String(plan.price):"",score=amount?0:-1;
 let rule:PriceRule|undefined;
 for(const item of rules){
 if(item.currency!==code||item.billing_period!==period||item.country&&item.country!==country)continue;
 const rank=item.country?2:1;if(rank>score){score=rank;amount=item.amount;rule=item;}
 }
 if(score<0)return null;
 let units=minor(amount,exp);
 if(rule?.decimal_precision!==undefined){
 const quantum=BigInt(10)**BigInt(exp-rule.decimal_precision),rest=units%quantum;
 if(rest){if(rule.rounding==="half_up")units+=rest>=quantum/BigInt(2)?quantum-rest:-rest;else if(rule.rounding==="down")units-=rest;else if(rule.rounding==="up")units+=quantum-rest;else return null;}
 }
 const tax=(units*BigInt(rule?.tax_bps||0)+BigInt(5000))/BigInt(10000),total=units+tax;
 if(total<=BigInt(0)||total>BigInt(Number.MAX_SAFE_INTEGER))return null;
 return {...plan,currency:code,billing_period:period,price:Number(major(total,exp)),subtotal:major(units,exp),tax:major(tax,exp),quote_total:major(total,exp)};
}
export function pricedPlans(plans:PublicPlan[],currency:string,country:string):QuotedPlan[]{
 const result:QuotedPlan[]=[];
 for(const plan of plans){
 if(plan.pricing_type==="custom"||Number(plan.price)===0){result.push({...plan,currency:currency||plan.currency});continue;}
 const periods=new Set([plan.billing_period,...(plan.billing_config?.prices||[]).map(p=>p.billing_period)]);
 for(const period of periods){
 let quoted:QuotedPlan|null=null;
 try{quoted=quotePlan(plan,currency,country,period);}catch{/* Preserve the card without offering an invalid quote. */}
 result.push(quoted||{...plan,currency:currency||plan.currency,billing_period:period,pricing_unavailable:true,subtotal:undefined,tax:undefined,quote_total:undefined});
 }
 }
 return result;
}
export function checkoutMethodOptions(methods?:string[]):Record<string,boolean>|undefined{
 if(!methods?.length)return undefined; // Let native Razorpay Checkout resolve actual merchant methods.
 return Object.fromEntries(["card","upi","netbanking","wallet","emi","paylater","bank_transfer"].map(id=>[id,methods.includes(id)]));
}

/** Format transaction and plan values using the currency's ISO decimal places. */
export function formatCurrencyAmount(value:number|string,code:string,language="en"):string {
 try {const digits=exponent(code);return Number(value).toLocaleString(language==="ar"?"ar":"en",{minimumFractionDigits:digits,maximumFractionDigits:digits});}
 catch {return String(value);}
}
