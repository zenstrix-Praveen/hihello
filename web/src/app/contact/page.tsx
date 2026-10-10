"use client";

import { FormEvent, useState } from "react";
import { ChevronDown, CircleHelp, Compass, Share2, UsersRound } from "lucide-react";
import PublicHeader from "@/components/public/PublicHeader";
import PublicFooter from "@/components/public/PublicFooter";
import { apiRequest } from "@/lib/user-api";
import { useWebsiteSection } from "@/components/public/useWebsiteSection";
import { useCustomerLocale } from "@/lib/customer-locale";

const questions = [
  ["What is a digital business card?", "A digital business card is a public online profile that can include your professional contact details, company information and useful links."],
  ["How do I create a digital card?", "Sign in to your customer workspace and choose Create Card. You can add supported details, choose a template and save or publish your card."],
  ["Can I use a template for my card?", "Yes. Browse the available templates and choose one as a starting point when creating or editing a digital card."],
  ["How do I share my card?", "Share the public card link, show its QR code, or use a compatible NFC card or tag configured with the link."],
  ["Do NFC and QR codes work on every phone?", "Device and browser support can vary. QR codes can be scanned with a compatible camera or reader, and NFC requires compatible hardware and settings."],
  ["Where can I manage my subscription?", "Open Plans & Billing in your customer workspace to review available plans, subscription details and payment history."],
  ["Where can I review a payment?", "Open Plans & Billing and select a recorded payment to review its available details. Checkout options depend on the payment provider configured for the platform."],
  ["How can I manage my account information?", "Open Profile in your customer workspace to review or update the account information supported there."],
  ["How can I contact customer support?", "Send a message using the form below. If a support email is listed on this page, you can also contact the team there."],
];

const aboutPoints = [
  { title: "Create", description: "Build a digital card with your professional details and a design that fits your identity.", icon: Compass },
  { title: "Share", description: "Share your public card using its link, QR code or configured NFC tag.", icon: Share2 },
  { title: "Connect", description: "Give visitors clear options to call, email, visit your website or save your public contact details.", icon: UsersRound },
];

export default function ContactPage() {
  const { t, error: localizeError } = useCustomerLocale();
  const cms = useWebsiteSection<any>("contact", {
    contact_page_heading: "Contact us",
    description: "Send a message and our team will follow up.",
    form_heading: "Send us a message",
    success_message: "Your message has been received.",
    support_email: "", sales_email: "", phone: "", address: "",
  }, { fallbackOnUnavailable: true });
  const [form, setForm] = useState({ name: "", email: "", phone: "", subject: "", message: "" });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");

  function set(field: keyof typeof form, value: string) {
    setForm(current => ({ ...current, [field]: value }));
    setError("");
    setSuccess("");
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    setSuccess("");
    setLoading(true);
    try {
      await apiRequest("/website/contact", { method: "POST", body: JSON.stringify(form) });
      setSuccess(t(cms.success_message || "Your message has been received."));
      setForm({ name: "", email: "", phone: "", subject: "", message: "" });
    } catch (cause) {
      setError(localizeError(cause));
    } finally {
      setLoading(false);
    }
  }

  const input = "mt-2 w-full rounded-xl border border-slate-300 bg-white px-4 py-3 text-slate-900 outline-none placeholder:text-slate-400 focus:border-blue-600 focus:ring-2 focus:ring-blue-100";
  const label = "block text-sm font-semibold text-slate-700";

  if (cms.enabled === false) return <><PublicHeader /><PublicFooter /></>;

  return <>
    <PublicHeader />
    <main className="min-h-[70vh] bg-slate-50 px-4 py-12 sm:px-6 sm:py-16">
      <div className="mx-auto max-w-5xl space-y-12 sm:space-y-16">
        <header className="mx-auto max-w-3xl text-center">
          <p className="text-sm font-bold uppercase tracking-[.18em] text-blue-600">{t("We’re here to help")}</p>
          <h1 className="mt-3 text-3xl font-black text-slate-950 sm:text-4xl">{cms.contact_page_heading}</h1>
          <p className="mx-auto mt-4 max-w-2xl leading-7 text-slate-600">{cms.description}</p>
          {(cms.support_email || cms.sales_email || cms.phone || cms.address || cms.business_hours) && <div className="mx-auto mt-6 grid max-w-3xl gap-3 rounded-2xl border border-slate-200 bg-white p-5 text-start text-sm text-slate-700 sm:grid-cols-2">
            {cms.support_email && <p>{t("Support")}: <a dir="ltr" className="text-blue-700 hover:underline" href={`mailto:${cms.support_email}`}>{cms.support_email}</a></p>}
            {cms.sales_email && <p>{t("Sales")}: <a dir="ltr" className="text-blue-700 hover:underline" href={`mailto:${cms.sales_email}`}>{cms.sales_email}</a></p>}
            {cms.phone && <p>{t("Phone")}: <a dir="ltr" className="text-blue-700 hover:underline" href={`tel:${cms.phone}`}>{cms.phone}</a></p>}
            {cms.address && <p>{t("Address")}: {cms.address}</p>}
            {cms.business_hours && <p>{t("Business hours")}: {cms.business_hours}</p>}
          </div>}
        </header>

        <section id="about-us" aria-labelledby="about-us-heading" className="scroll-mt-28 overflow-hidden rounded-3xl border border-blue-100 bg-white shadow-sm">
          <div className="bg-gradient-to-br from-blue-700 to-indigo-700 px-6 py-8 text-white sm:px-10 sm:py-10">
            <p className="text-sm font-bold uppercase tracking-[.16em] text-blue-100">{t("About Dbizcard")}</p>
            <h2 id="about-us-heading" className="mt-2 text-2xl font-black sm:text-3xl">{t("Know About Us")}</h2>
            <p className="mt-4 max-w-3xl text-base leading-7 text-blue-50 sm:text-lg">{t("Dbizcard helps people and teams create, manage and share digital business cards. Keep your public contact information together and make it easy for others to connect.")}</p>
          </div>
          <div className="grid gap-4 p-5 sm:grid-cols-3 sm:p-7">
            {aboutPoints.map(({ title, description, icon: Icon }) => <article key={title} className="rounded-2xl border border-slate-200 bg-slate-50 p-5">
              <span className="grid h-10 w-10 place-items-center rounded-xl bg-blue-100 text-blue-700"><Icon aria-hidden="true" className="h-5 w-5" /></span>
              <h3 className="mt-4 text-lg font-bold text-slate-900">{t(title)}</h3>
              <p className="mt-2 text-sm leading-6 text-slate-600">{t(description)}</p>
            </article>)}
          </div>
        </section>

        <section id="faq" aria-labelledby="faq-heading" className="scroll-mt-28">
          <div className="mx-auto max-w-3xl text-center">
            <span className="mx-auto grid h-11 w-11 place-items-center rounded-2xl bg-blue-100 text-blue-700"><CircleHelp aria-hidden="true" className="h-6 w-6" /></span>
            <h2 id="faq-heading" className="mt-4 text-2xl font-black text-slate-950 sm:text-3xl">{t("Frequently Asked Questions")}</h2>
            <p className="mt-3 leading-7 text-slate-600">{t("Answers to common questions about digital cards, sharing and billing.")}</p>
          </div>
          <div className="mx-auto mt-7 max-w-4xl space-y-3">
            {questions.map(([question, answer]) => <details key={question} className="group rounded-2xl border border-slate-200 bg-white px-5 shadow-sm open:border-blue-200 open:shadow-md sm:px-6">
              <summary className="flex min-h-16 cursor-pointer list-none items-center justify-between gap-4 py-4 text-start font-bold text-slate-900 marker:content-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-600 [&::-webkit-details-marker]:hidden">
                <span>{t(question)}</span>
                <ChevronDown aria-hidden="true" className="h-5 w-5 shrink-0 text-blue-600 transition-transform group-open:rotate-180" />
              </summary>
              <p className="max-w-3xl pb-5 leading-7 text-slate-600">{t(answer)}</p>
            </details>)}
          </div>
        </section>

        <section id="send-message" aria-labelledby="send-message-heading" className="scroll-mt-28 mx-auto max-w-3xl rounded-3xl border border-slate-200 bg-white p-5 shadow-sm sm:p-9">
          <div className="mb-7 text-center">
            <p className="text-sm font-bold uppercase tracking-[.16em] text-blue-700">{t("Contact us")}</p>
            <h2 id="send-message-heading" className="mt-2 text-2xl font-black text-slate-950">{t(cms.form_heading || "Send us a message")}</h2>
          </div>
          <form onSubmit={submit} className="space-y-5">
            <div className="grid gap-5 sm:grid-cols-2">
              <label className={label}>{t("Name")} *<input className={input} required maxLength={150} autoComplete="name" value={form.name} onChange={event => set("name", event.target.value)} /></label>
              <label className={label}>{t("Email")} *<input className={input} required type="email" maxLength={255} autoComplete="email" dir="ltr" value={form.email} onChange={event => set("email", event.target.value)} /></label>
              <label className={label}>{t("Phone")} <span className="font-normal text-slate-500">({t("optional")})</span><input className={input} type="tel" maxLength={50} autoComplete="tel" dir="ltr" value={form.phone} onChange={event => set("phone", event.target.value)} /></label>
              <label className={label}>{t("Subject / Purpose")} *<input className={input} required maxLength={200} value={form.subject} onChange={event => set("subject", event.target.value)} /></label>
            </div>
            <label className={label}>{t("Message")} *<textarea className={`${input} min-h-36 resize-y`} required maxLength={20000} value={form.message} onChange={event => set("message", event.target.value)} /></label>
            {error && <p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{error}</p>}
            {success && <p role="status" className="rounded-xl bg-emerald-50 p-3 text-sm text-emerald-800">{success}</p>}
            <button disabled={loading} className="w-full rounded-xl bg-blue-600 px-5 py-3.5 font-bold text-white transition hover:bg-blue-700 disabled:cursor-wait disabled:opacity-60">{loading ? t("Sending…") : t("Send Message")}</button>
          </form>
        </section>
      </div>
    </main>
    <PublicFooter />
  </>;
}
