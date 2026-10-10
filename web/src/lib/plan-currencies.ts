export type PlanCurrencyOption={code:string;name:string;symbol?:string;exponent?:number};

export function searchPlanCurrencies(query: string, t: (key:string)=>string, currencies:PlanCurrencyOption[], existingCurrency?:string) {
  const options: PlanCurrencyOption[] = [...currencies];
  if (existingCurrency && !options.some(item=>item.code===existingCurrency)) options.push({code:existingCurrency,name:'Existing currency'});
  const search=query.trim().toLocaleLowerCase();
  return options.filter(item=>`${item.code} ${item.name} ${t(item.name)}`.toLocaleLowerCase().includes(search));
}
