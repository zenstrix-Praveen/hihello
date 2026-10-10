import Link from "next/link";

const modules = [
  {
    title: "Home",
    description: "Manage hero section, headline, CTA buttons and home page sections.",
    href: "/admin/website/home",
  },
  {
    title: "Features",
    description: "Manage features shown on the public website.",
    href: "/admin/website/features",
  },
  {
    title: "Templates",
    description: "Manage digital card templates displayed publicly.",
    href: "/admin/website/templates",
  },
  {
    title: "Pricing",
    description: "Manage plans, prices, billing periods and feature lists.",
    href: "/admin/website/pricing",
  },
  {
    title: "Business / Enterprise",
    description: "Manage enterprise content, business benefits and CTA.",
    href: "/admin/website/business",
  },
  {
    title: "Contact",
    description: "Manage contact details and enquiries.",
    href: "/admin/website/contact",
  },
  {
    title: "Header / Navigation",
    description: "Manage logo, navigation links and primary actions.",
    href: "/admin/website/header",
  },
  {
    title: "Footer",
    description: "Manage footer links, company information and social links.",
    href: "/admin/website/footer",
  },
  {
    title: "SEO / General Settings",
    description: "Manage site name, metadata, favicon and SEO settings.",
    href: "/admin/website/seo",
  },
];

export default function WebsiteAdminPage() {
  return (
    <div>
      <div>
        <p className="text-sm font-medium text-blue-600">
          Admin module
        </p>

        <h1 className="mt-1 text-3xl font-bold text-slate-900">
          Website
        </h1>

        <p className="mt-2 text-sm text-slate-500">
          Manage the content displayed on the public website.
        </p>
      </div>

      <div className="mt-8 grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-3">
        {modules.map((module) => (
          <Link
            href={module.href}
            key={module.href}
            className="rounded-2xl border border-slate-200 bg-white p-6 transition hover:border-blue-300 hover:shadow-sm"
          >
            <h2 className="text-lg font-semibold text-slate-900">
              {module.title}
            </h2>

            <p className="mt-2 text-sm leading-6 text-slate-500">
              {module.description}
            </p>

            <p className="mt-5 text-sm font-semibold text-blue-600">
              Manage →
            </p>
          </Link>
        ))}
      </div>
    </div>
  );
}
