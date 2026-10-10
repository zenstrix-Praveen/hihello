import CardDetail from "@/components/dashboard/CardDetail";
export default async function EditCardPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <CardDetail id={id} mode="edit" />;
}
