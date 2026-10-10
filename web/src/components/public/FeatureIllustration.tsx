"use client";

import { useState } from "react";
import { ContactRound } from "lucide-react";
import { useCustomerLocale } from "@/lib/customer-locale";

export default function FeatureIllustration({ src, alt }: { src: string; alt: string }) {
  const { t } = useCustomerLocale();
  const [failed, setFailed] = useState(false);
  return <span className="relative block aspect-[4/3] w-full overflow-hidden rounded-[20px] bg-gradient-to-br from-blue-50 to-cyan-50">
    {failed ? <span role="img" aria-label={t(alt)} className="absolute inset-0 flex flex-col items-center justify-center gap-3 p-6 text-center text-blue-700"><ContactRound className="h-16 w-16" aria-hidden="true" /><span className="text-sm">{t("Your digital identity, connected.")}</span></span> : <img src={src} alt={t(alt)} width={800} height={600} loading="lazy" onError={() => setFailed(true)} className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.02] motion-reduce:transform-none motion-reduce:transition-none" />}
  </span>;
}
