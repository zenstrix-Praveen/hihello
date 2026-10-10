"use client";

import { useCustomerLocale } from "@/lib/customer-locale";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { Bot, ChevronDown, MessageCircle, Minus, Send, Sparkles, Trash2, X } from "lucide-react";
import { apiRequest } from "@/lib/user-api";
import { chatShortcuts, fallbackAnswer, localKnowledgeProvider, quickQuestions, type ChatAction, type ChatAudience, type CustomerChatContext } from "@/lib/chatbot-knowledge";

type Message = { id: number; role: "assistant" | "user"; text: string; translationKey?: string; params?: Record<string, string | number>; actions?: ChatAction[] };
const welcome = "Hi! I'm the Dbizcard Assistant. How can I help you today?";
const makeWelcome = (): Message[] => [{ id: 0, role:"assistant", text:welcome }];

export function ChatbotLauncher({ audience, customerName }: { audience: ChatAudience; customerName?: string }) {
  const { t, error: localizeError } = useCustomerLocale();
  const [open, setOpen] = useState(false);
  const [minimized, setMinimized] = useState(false);
  const [messages, setMessages] = useState<Message[]>(makeWelcome);
  const [input, setInput] = useState("");
  const [context, setContext] = useState<CustomerChatContext | undefined>(audience === "customer" ? { name:customerName } : undefined);
  const contextLoaded = useRef(false);
  const [contextError, setContextError] = useState("");
  const [busy, setBusy] = useState(false);
  const launcher = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const endRef = useRef<HTMLDivElement>(null);
  const counter = useRef(1);
  const [mobileTop, setMobileTop] = useState(80);

  useEffect(() => {
    if (!open) return;
    inputRef.current?.focus();
    const keydown = (event: KeyboardEvent) => {
      if (event.key === "Escape") { event.preventDefault(); setOpen(false); setMinimized(false); window.setTimeout(() => launcher.current?.focus(), 0); }
      if (event.key !== "Tab" || !panel.current || minimized) return;
      const elements = [...panel.current.querySelectorAll<HTMLElement>('button:not([disabled]),a[href],input:not([disabled])')];
      if (!elements.length) return;
      if (event.shiftKey && document.activeElement === elements[0]) { event.preventDefault(); elements[elements.length - 1].focus(); }
      else if (!event.shiftKey && document.activeElement === elements[elements.length - 1]) { event.preventDefault(); elements[0].focus(); }
    };
    window.addEventListener("keydown", keydown);
    return () => window.removeEventListener("keydown", keydown);
  }, [open, minimized]);

  useEffect(() => {
    const closeChat = () => { setOpen(false); setMinimized(false); };
    window.addEventListener("dbizcard:close-chat", closeChat);
    return () => window.removeEventListener("dbizcard:close-chat", closeChat);
  }, []);

  useEffect(() => { endRef.current?.scrollIntoView({ block:"nearest" }); }, [messages, open, minimized]);

  useEffect(() => {
    if (!open || audience !== "customer" || contextLoaded.current) return;
    let alive = true;
    contextLoaded.current = true;
    apiRequest("/user/dashboard").then((data: { total_cards?: number; published_cards?: number; draft_cards?: number; current_plan?: { name?: string } | null }) => {
      if (!alive) return;
      setContextError("");
      setContext(current => ({ ...current, planName:data.current_plan?.name, cardCount:Number.isFinite(data.total_cards) ? data.total_cards : undefined, publishedCards:Number.isFinite(data.published_cards) ? data.published_cards : undefined, draftCards:Number.isFinite(data.draft_cards) ? data.draft_cards : undefined }));
    }).catch(error => { if (alive) setContextError(localizeError(error)); });
    return () => { alive = false; };
  }, [open, audience, localizeError]);

  useEffect(() => {
    if (!open || audience !== "public") return;
    const header = document.querySelector<HTMLElement>('[data-public-header]');
    if (!header) return;
    const updateTop = () => setMobileTop(Math.max(0, Math.min(window.innerHeight, header.getBoundingClientRect().bottom)));
    updateTop();
    // Locale notices and the closing menu can both change the header's position.
    const observer = new ResizeObserver(updateTop);
    observer.observe(header);
    if (header.parentElement) observer.observe(header.parentElement);
    window.addEventListener("resize", updateTop);
    window.addEventListener("scroll", updateTop, true);
    return () => {
      observer.disconnect();
      window.removeEventListener("resize", updateTop);
      window.removeEventListener("scroll", updateTop, true);
    };
  }, [open, audience]);

  function answer(question: string) {
    const clean = question.trim();
    if (!clean || busy) return;
    setBusy(true);
    const userMessage: Message = { id:counter.current++, role:"user", text:clean };
    const result = localKnowledgeProvider.answer(clean, audience, context) || fallbackAnswer;
    const assistantMessage: Message = { id:counter.current++, role:"assistant", text:result.answer, translationKey:result.translationKey, params:result.params, actions:result.actions };
    setMessages(existing => [...existing, userMessage, assistantMessage]);
    setInput("");
    setBusy(false);
  }

  function resetChat() { counter.current = 1; setMessages(makeWelcome()); setInput(""); }
  function closePanel() { setOpen(false); setMinimized(false); window.setTimeout(() => launcher.current?.focus(), 0); }

  return <>
    {open && <section ref={panel} role="dialog" aria-modal="false" aria-labelledby="dbizcard-chat-title" style={{ "--dbizcard-chat-top": `${mobileTop}px` } as CSSProperties} className={`fixed z-[90] flex flex-col overflow-hidden border border-slate-200 bg-white shadow-[0_24px_80px_-20px_rgba(15,23,42,.38)] ${minimized ? "bottom-[calc(env(safe-area-inset-bottom)+5.5rem)] right-4 h-auto w-[calc(100vw-2rem)] max-w-[390px] rounded-2xl min-[1280px]:bottom-24 min-[1280px]:right-6" : "inset-x-0 top-[var(--dbizcard-chat-top)] bottom-0 h-[calc(100dvh-var(--dbizcard-chat-top))] rounded-none min-[1280px]:inset-auto min-[1280px]:top-auto min-[1280px]:bottom-24 min-[1280px]:right-6 min-[1280px]:h-[min(640px,calc(100dvh-8rem))] min-[1280px]:w-[min(400px,calc(100vw-3rem))] min-[1280px]:rounded-[24px]"}`}>
      <header className="flex min-h-[76px] items-center gap-3 bg-gradient-to-r from-blue-700 via-blue-600 to-indigo-600 px-4 py-3 text-white">
        <span className="grid h-11 w-11 shrink-0 place-items-center rounded-2xl bg-white/15"><Bot aria-hidden="true" className="h-6 w-6" /></span>
        <div className="min-w-0 flex-1"><h2 id="dbizcard-chat-title" className="font-bold">{t("Dbizcard Assistant")}</h2><p className="truncate text-xs text-blue-100">{t("Ask about cards, QR, NFC, plans and your account.")}</p></div>
        <button type="button" onClick={() => setMinimized(value => !value)} aria-label={minimized ? t("Expand chat") : t("Minimize chat")} className="grid h-11 w-11 shrink-0 place-items-center rounded-lg text-white/90 hover:bg-white/15 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white">{minimized ? <ChevronDown aria-hidden="true" className="h-5 w-5" /> : <Minus aria-hidden="true" className="h-5 w-5" />}</button>
        <button type="button" onClick={closePanel} aria-label={t("Close chat")} className="grid h-11 w-11 shrink-0 place-items-center rounded-lg text-white/90 hover:bg-white/15 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white"><X aria-hidden="true" className="h-5 w-5" /></button>
      </header>
      {!minimized && <>
        <div className="flex items-center justify-between border-b border-slate-100 px-4 py-2"><span className="text-xs font-medium text-slate-500">{t("Guided product help · no AI data sharing")}</span><button type="button" onClick={resetChat} className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-semibold text-slate-500 hover:bg-slate-100 hover:text-slate-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600"><Trash2 aria-hidden="true" className="h-3.5 w-3.5"/>{t("Clear Chat")}</button></div>
        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto bg-slate-50/80 p-4" aria-live="polite" aria-relevant="additions text">
          {contextError && audience === "customer" && <p role="status" className="rounded-xl border border-amber-200 bg-amber-50 p-3 text-xs leading-5 text-amber-900">{t("Account details could not be loaded. General support answers are still available.")} {contextError}</p>}
          {messages.map(message => <ChatMessage key={message.id} message={message} />)}
          {messages.length === 1 && <div className="space-y-2"><p className="px-1 text-xs font-bold uppercase tracking-wider text-slate-500">{t("Quick questions")}</p><div className="flex flex-wrap gap-2">{quickQuestions.map(question => <button key={question} type="button" onClick={() => question === "Contact Support" ? answer(t("Contact Support")) : answer(t(question))} className="rounded-full border border-blue-100 bg-white px-3 py-2 text-start text-xs font-semibold text-blue-800 shadow-sm hover:border-blue-300 hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600">{t(question)}</button>)}</div></div>}
          <div ref={endRef} />
        </div>
        <nav aria-label={audience === "customer" ? t("Customer shortcuts") : t("Website shortcuts")} className="flex gap-2 overflow-x-auto border-t border-slate-100 bg-white px-3 py-2">{chatShortcuts[audience].map(action => <Link key={action.href} href={action.href} onClick={closePanel} className="shrink-0 rounded-full bg-slate-100 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-blue-50 hover:text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600">{t(action.label)}</Link>)}</nav>
        <form onSubmit={event => { event.preventDefault(); answer(input); }} className="flex items-center gap-2 border-t border-slate-100 bg-white p-3 pb-[calc(.75rem+env(safe-area-inset-bottom))] sm:pb-3"><label className="sr-only" htmlFor="dbizcard-chat-input">{t("Ask Dbizcard Assistant")}</label><input ref={inputRef} id="dbizcard-chat-input" value={input} onChange={event => setInput(event.target.value)} maxLength={500} placeholder={t("Type your question…")} autoComplete="off" className="min-w-0 flex-1 rounded-xl border border-slate-200 px-4 py-3 text-sm text-slate-900 outline-none placeholder:text-slate-400 focus:border-blue-500 focus:ring-2 focus:ring-blue-100"/><button type="submit" disabled={!input.trim() || busy} aria-label={t("Send message")} className="grid h-11 w-11 shrink-0 place-items-center rounded-xl bg-blue-600 text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600 focus-visible:ring-offset-2"><Send aria-hidden="true" className="h-4 w-4" /></button></form>
      </>}
    </section>}
    <button ref={launcher} type="button" onClick={() => { window.dispatchEvent(new Event("dbizcard:close-navigation")); setOpen(true); setMinimized(false); }} aria-label={open ? t("Open Dbizcard Assistant") : t("Open support chat")} aria-expanded={open} className={`fixed bottom-[calc(env(safe-area-inset-bottom)+1rem)] right-4 z-[91] grid h-14 w-14 touch-manipulation select-none place-items-center rounded-full bg-gradient-to-br from-blue-600 to-indigo-600 text-white shadow-[0_12px_30px_-8px_rgba(37,99,235,.75)] transition hover:scale-105 hover:shadow-xl focus-visible:outline-none focus-visible:ring-4 focus-visible:ring-blue-300 sm:bottom-6 sm:right-6 ${open ? "hidden" : ""}`}><MessageCircle aria-hidden="true" className="h-6 w-6"/><span className="sr-only">{audience === "customer" && customerName ? t("Support chat for {name}", {name: customerName}) : t("Dbizcard support chat")}</span></button>
  </>;
}

function ChatMessage({ message }: { message: Message }) {
  const { t } = useCustomerLocale();
  const assistant = message.role === "assistant";
  return <div className={`flex ${assistant ? "justify-start" : "justify-end"}`}><div className={`max-w-[90%] rounded-2xl px-4 py-3 text-sm leading-6 ${assistant ? "rounded-bl-md border border-slate-200 bg-white text-slate-700 shadow-sm" : "rounded-br-md bg-blue-600 text-white"}`}>
    {assistant && <p className="mb-1 flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-wide text-blue-700"><Sparkles aria-hidden="true" className="h-3 w-3"/>{t("Dbizcard Assistant")}</p>}
    <p className="whitespace-pre-wrap">{assistant ? t(message.translationKey || message.text, message.params) : message.text}</p>
    {assistant && message.actions && message.actions.length > 0 && <div className="mt-3 flex flex-wrap gap-2">{message.actions.map(action => <Link key={`${action.label}-${action.href}`} href={action.href} className="inline-flex items-center rounded-lg border border-blue-100 bg-blue-50 px-3 py-1.5 text-xs font-bold text-blue-800 hover:bg-blue-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-600">{action.label}</Link>)}</div>}
  </div></div>;
}
