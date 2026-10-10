import ResourceList from "@/components/dashboard/ResourceList";
export default function ContactsPage() {return <ResourceList title="Contacts" kind="contacts" columns={[["name","Name"],["email","Email"],["phone","Phone"],["source","Source"]]}/>;}
