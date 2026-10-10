"use client";

import Image from "next/image";
import { useState } from "react";

type BrandLogoProps = { className?: string; imageClassName?: string; priority?: boolean };

export default function BrandLogo({ className = "", imageClassName = "", priority = false }: BrandLogoProps) {
  const [failed, setFailed] = useState(false);
  return (
    <span className={`relative inline-flex shrink-0 items-center ${className}`}>
      {!failed ? <Image
        src="/logo.png"
        alt="DBIZCARD — Digital Business Card"
        width={1942}
        height={809}
        priority={priority}
        sizes="(max-width: 639px) 160px, 190px"
        onError={() => setFailed(true)}
        className={`block h-auto max-h-full w-full object-contain ${imageClassName}`}
      /> : <span className="text-xl font-extrabold tracking-tight text-slate-900">DBIZCARD</span>}
    </span>
  );
}
