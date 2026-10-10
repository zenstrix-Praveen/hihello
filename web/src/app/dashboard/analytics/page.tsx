import Analytics from "@/components/dashboard/Analytics";
export default async function AnalyticsPage({searchParams}:{searchParams:Promise<{card?:string}>}) { const {card}=await searchParams;return <Analytics cardID={card}/>;}
