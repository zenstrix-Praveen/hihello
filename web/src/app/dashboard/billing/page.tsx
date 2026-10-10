import Billing from "@/components/dashboard/Billing";
export default async function BillingPage({searchParams}:{searchParams:Promise<{plan?:string;billing_period?:string;currency?:string;country?:string;reviewed_amount?:string}>}) {
 const {plan,billing_period,currency,country,reviewed_amount}=await searchParams;
 return <Billing planID={plan} billingPeriod={billing_period} currency={currency} country={country} reviewedAmount={reviewed_amount}/>;
}
