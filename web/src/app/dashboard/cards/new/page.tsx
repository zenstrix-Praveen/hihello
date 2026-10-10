import CardEditor from "@/components/dashboard/CardEditor";
export default async function NewCardPage({ searchParams }: { searchParams: Promise<{ template?: string }> }) {
  const { template } = await searchParams;
  return <CardEditor template={template} />;
}
