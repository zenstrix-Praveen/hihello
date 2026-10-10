"use client";
import { useCustomerLocale } from "@/lib/customer-locale";

import { useEffect, useState } from "react";
import { apiRequest } from "@/lib/user-api";
import type { Card } from "./data";

export default function CardSharePanel({ card }: { card: Card }) {
  const { t, error: localizeError } = useCustomerLocale();
  const published = card.status === "published";
  const [qrReady, setQrReady] = useState(false);
  const [qrImage, setQrImage] = useState("");
  const [qrError, setQrError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [notice, updateNotice] = useState<{ message: string; params?: Record<string, string | number> }>({ message: "" });
  const setNotice = (message: string, params?: Record<string, string | number>) => updateNotice({ message, params });
  const [emailOpen, setEmailOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [message, setMessage] = useState("");
  const [sending, setSending] = useState(false);
  const [publicURL, setPublicURL] = useState("");

  useEffect(() => {
    setPublicURL(`${window.location.origin}/c/${encodeURIComponent(card.slug)}`);
    setQrReady(false);
    setQrImage("");
    setQrError("");
    if (!published) return;
    let live = true;
    apiRequest(`/user/cards/${card.id}/qr`, { method: "POST" })
      .then((result) => {
        const image = result?.data?.qr_image;
        if (!image || typeof image !== "string" || !image.startsWith("data:image/png;base64,")) throw new Error("QR image missing");
        if (live) { setQrImage(image); setQrReady(true); }
      })
      .catch(() => { if (live) setQrError("Unable to load QR code. Try again."); });
    return () => { live = false; };
  }, [card.id, card.slug, published, attempt]);

  useEffect(() => {
    if (!emailOpen) return;
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === "Escape") setEmailOpen(false); };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [emailOpen]);

  async function copyLink() {
    try {
      await navigator.clipboard.writeText(publicURL);
      setNotice("Card link copied.");
    } catch {
      setNotice("Copy this card link: {url}", { url: publicURL });
    }
  }

  async function downloadQR() {
    try {
      const response = await fetch(qrImage);
      const blob = await response.blob();
      if (!blob.size || blob.type !== "image/png") throw new Error("Invalid QR image");
      const objectURL = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = objectURL;
      anchor.download = `dbizcard-${card.slug}-qr.png`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000);
      setNotice("QR code downloaded.");
    } catch {
      setNotice("Unable to download QR code.");
    }
  }

  async function shareCard() {
    try {
      const response = await fetch(qrImage);
      const blob = await response.blob();
      if (!blob.size || blob.type !== "image/png") throw new Error("Invalid QR image");
      const file = new File([blob], `dbizcard-${card.slug}-qr.png`, { type: "image/png" });
      if (navigator.share && navigator.canShare?.({ files: [file] })) {
        await navigator.share({ title: `${card.name} · Dbizcard`, text: t("Scan to view my digital business card."), url: publicURL, files: [file] });
      } else if (navigator.share) {
        await navigator.share({ title: `${card.name} · Dbizcard`, text: t("View my digital business card."), url: publicURL });
      } else {
        await copyLink();
        setNotice("Sharing is unavailable here. Card link copied; you can also download the QR code.");
      }
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
      try {
        await navigator.clipboard.writeText(publicURL);
        setNotice("QR sharing is unavailable here. Card link copied; you can also download the QR code.");
      } catch {
        setNotice("Sharing is unavailable here. Copy this link or download the QR code: {url}", { url: publicURL });
      }
    }
  }

  async function sendEmail(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (sending) return;
    setSending(true);
    try {
      await apiRequest(`/user/cards/${card.id}/share/email`, { method: "POST", body: JSON.stringify({ email, message }) });
      setEmailOpen(false);
      setEmail("");
      setMessage("");
      setNotice("Card email sent.");
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Could not send card email.");
    } finally {
      setSending(false);
    }
  }

  return <section className="rounded-3xl border border-slate-200 bg-white p-5 shadow-lg shadow-slate-900/5 sm:p-7">
    <div><p className="text-xs font-bold uppercase tracking-[.18em] text-blue-700">{t("Share your card")}</p><h2 className="mt-2 text-2xl font-bold text-slate-900">{t("Ready to connect?")}</h2><p className="mt-2 text-sm leading-6 text-slate-600">{t("Share a link or let someone scan your personal QR code.")}</p></div>
    {!published ? <div className="mt-6 rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-900"><strong>{card.status === "archived" ? t("This card is archived.") : t("Publish this card to enable sharing.")}</strong><br/>{card.status === "archived" ? t("Restore and publish it before sharing.") : t("Draft cards stay private. Publish it from Edit Card when it is ready.")}</div> : <>
      <div className="mx-auto mt-6 grid min-h-64 w-full max-w-72 place-items-center rounded-2xl border border-slate-200 bg-white p-4 shadow-inner">
        {qrReady ? <img src={qrImage} alt={t("QR code linking to {name}’s public card", { name: card.name })} className="h-60 w-60 bg-white object-contain" /> : <div className="px-3 text-center text-sm text-slate-500">{qrError ? localizeError(qrError) : t("Preparing your QR code…")}{qrError && <button type="button" onClick={() => { setQrError(""); setAttempt(value => value + 1); }} className="mt-3 block w-full font-semibold text-blue-700">{t("Try again")}</button>}</div>}
      </div>
      {qrError && <p role="alert" className="mt-2 text-center text-sm text-red-700">{localizeError(qrError)}</p>}
      <div className="mt-5 grid gap-2 sm:grid-cols-2">
        <a href={publicURL || undefined} target="_blank" rel="noreferrer" aria-disabled={!publicURL} className="rounded-xl bg-blue-600 px-4 py-3 text-center text-sm font-semibold text-white hover:bg-blue-700">{t("Open Public Card")}</a>
        <button type="button" onClick={copyLink} className="rounded-xl border border-blue-200 px-4 py-3 text-sm font-semibold text-blue-800 hover:bg-blue-50">{t("Copy Link")}</button>
        <button type="button" disabled={!qrReady} onClick={downloadQR} className="rounded-xl border border-slate-200 px-4 py-3 text-sm font-semibold text-slate-800 hover:bg-slate-50 disabled:opacity-50">{t("Download QR")}</button>
        <button type="button" disabled={!qrReady} onClick={shareCard} className="rounded-xl border border-slate-200 px-4 py-3 text-sm font-semibold text-slate-800 hover:bg-slate-50 disabled:opacity-50">{t("Share QR")}</button>
        <button type="button" onClick={() => setEmailOpen(true)} className="rounded-xl bg-gradient-to-r from-blue-600 to-indigo-600 px-4 py-3 text-sm font-semibold text-white shadow-sm hover:from-blue-700 hover:to-indigo-700">{t("Email Card")}</button>
      </div>
      {publicURL && <p dir="ltr" className="mt-3 break-all rounded-xl bg-slate-50 p-3 text-xs text-slate-600">{publicURL}</p>}
      <p className="mt-3 text-xs leading-5 text-slate-500">{t("The QR opens your public card through Dbizcard’s scan link, so scans can be counted without exposing private card data.")}</p>
    </>}
    {notice.message && <p role="status" className="mt-4 rounded-xl bg-blue-50 p-3 text-sm text-blue-800">{notice.params ? t(notice.message, notice.params) : localizeError(notice.message)}</p>}
    {emailOpen && <div className="fixed inset-0 z-50 grid place-items-center bg-slate-950/50 p-4 backdrop-blur-sm" onMouseDown={event => { if (event.target === event.currentTarget) setEmailOpen(false); }}>
      <section role="dialog" aria-modal="true" aria-labelledby="share-email-title" className="w-full max-w-lg rounded-3xl bg-white p-6 shadow-2xl sm:p-8">
        <div className="flex items-start justify-between gap-4"><div><h2 id="share-email-title" className="text-xl font-bold">{t("Share Card by Email")}</h2><p className="mt-1 text-sm text-slate-500">{t("We’ll send your public card link to the recipient.")}</p></div><button type="button" aria-label={t("Close email dialog")} onClick={() => setEmailOpen(false)} className="rounded-lg px-3 py-2 text-slate-600 hover:bg-slate-100">✕</button></div>
        <form onSubmit={sendEmail} className="mt-5 space-y-4"><label className="block text-sm font-semibold">{t("Recipient Email")}<input dir="ltr" autoFocus required type="email" maxLength={254} value={email} onChange={event => setEmail(event.target.value)} className="mt-2 w-full rounded-xl border border-slate-300 px-3 py-3 font-normal outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-100" /></label><label className="block text-sm font-semibold">{t("Optional Message")}<textarea maxLength={500} rows={4} value={message} onChange={event => setMessage(event.target.value)} className="mt-2 w-full resize-y rounded-xl border border-slate-300 px-3 py-3 font-normal outline-none focus:border-blue-600 focus:ring-2 focus:ring-blue-100" placeholder={t("Add a short note")} /></label><div className="flex justify-end gap-3"><button type="button" onClick={() => setEmailOpen(false)} className="rounded-xl border px-4 py-3 text-sm font-semibold">{t("Cancel")}</button><button disabled={sending} className="rounded-xl bg-blue-600 px-5 py-3 text-sm font-semibold text-white disabled:opacity-50">{sending ? t("Sending…") : t("Send Card")}</button></div></form>
      </section>
    </div>}
  </section>;
}
