"use client";
import {usePathname} from "next/navigation";
import {useEffect,useState} from "react";

export function useAdminLocation(){
 const pathname=usePathname();const [search,setSearch]=useState("");
 useEffect(()=>{
  const sync=()=>setSearch(window.location.search.slice(1));
  const navigate=(event:Event)=>{const href=(event as CustomEvent<string>).detail;if(href)setSearch(href.split("?")[1]??"");else sync()};
  sync();window.addEventListener("popstate",sync);window.addEventListener("admin:navigate",navigate);return()=>{window.removeEventListener("popstate",sync);window.removeEventListener("admin:navigate",navigate)}
 },[pathname]);
 return {pathname,search,params:new URLSearchParams(search)};
}
