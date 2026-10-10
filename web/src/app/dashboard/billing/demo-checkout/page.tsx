import DemoCheckout from "@/components/dashboard/DemoCheckout";

export default async function DemoCheckoutPage({ searchParams }: { searchParams: Promise<{ plan?: string }> }) {
  const { plan } = await searchParams;
  return <DemoCheckout planID={plan} />;
}
