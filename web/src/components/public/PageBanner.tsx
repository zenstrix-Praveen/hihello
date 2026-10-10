export default function PageBanner({
  title,
  breadcrumb,
}: {
  title: string;
  breadcrumb: string;
}) {
  return (
    <section className="bg-slate-900 py-16 text-white">
      <div className="mx-auto max-w-7xl px-5">
        <h1 className="text-4xl font-black">{title}</h1>
        <p className="mt-3 text-slate-300">{breadcrumb}</p>
      </div>
    </section>
  );
}
