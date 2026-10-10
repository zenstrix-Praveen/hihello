"use client";
import {useCallback,useEffect,useRef,useState} from "react";
import {adminApi} from "@/lib/admin-api";
import {useAdminLocation} from "@/lib/admin-location";
type Org={id:number;public_id:string;name:string;email:string|null;status:string;created_at:string;card_count:number;user_count:number};
type OrgDetail=Org&{users:{id:number;name:string;email:string;status:string}[];cards:{id:number;name:string;slug:string;status:string;created_at:string}[]};
type Result={data:Org[];pagination:{page:number;limit:number;total:number;pages:number}};
type Draft={name:string;email:string;status:string}; const empty:Draft={name:"",email:"",status:"active"};
export default function OrganizationsPage(){
 const {search:routeSearch}=useAdminLocation();
 const [result,setResult]=useState<Result|null>(null),[page,setPage]=useState(1),[search,setSearch]=useState(""),[status,setStatus]=useState("");
 const [loading,setLoading]=useState(true),[error,setError]=useState(""),[message,setMessage]=useState("");
 const [modal,setModal]=useState(false),[editing,setEditing]=useState<Org|null>(null),[draft,setDraft]=useState<Draft>(empty),[saving,setSaving]=useState(false),[detail,setDetail]=useState<OrgDetail|null>(null),[detailLoading,setDetailLoading]=useState(false);
 const [deleting,setDeleting]=useState(false),[deleteTarget,setDeleteTarget]=useState<Org|null>(null);
 const deleteBusy=useRef(false),listRequest=useRef(0);
 const load=useCallback(async()=>{
  const request=++listRequest.current;
  setLoading(true);setError("");
  try{
   let requestedPage=page;
   let next:Result;
   for(;;){
    const q=new URLSearchParams({page:String(requestedPage),limit:"20",search,status});
    next=await adminApi(`/admin/organizations?${q}`);
    if(request!==listRequest.current)return false;
    const lastPage=Math.max(next.pagination.pages,1);
    if(requestedPage<=lastPage)break;
    requestedPage=lastPage;
   }
   setResult(next);setPage(requestedPage);return true;
  }catch(e){if(request===listRequest.current)setError(e instanceof Error?e.message:"Could not load organizations");return false;}
  finally{if(request===listRequest.current)setLoading(false);}
 },[page,search,status]);useEffect(()=>{void load();},[load]);
 const latestLoad=useRef(load);latestLoad.current=load;
 function confirmDelete(o:Org){if(deleteBusy.current||loading||saving)return;setError("");setMessage("");setDeleteTarget(o);}
 async function removeOrganization(){
  if(!deleteTarget||deleteBusy.current)return;
  deleteBusy.current=true;listRequest.current+=1;setDeleting(true);setError("");setMessage("");
  try{
   await adminApi(`/admin/organizations/${deleteTarget.id}`,{method:"DELETE"});
   listRequest.current+=1;
   setDeleteTarget(null);setResult(null);
   setDetail(current=>current?.id===deleteTarget.id?null:current);
   setMessage("Organization deleted successfully.");
   if(!await latestLoad.current())setError("Organization was deleted, but the list could not be reloaded. Retry to load organizations.");
  }catch(e){
   const reason=e instanceof Error?e.message:"";
   setError(reason.startsWith("This organization cannot be deleted")||reason==="Organization not found"||reason==="Permission denied"||reason==="Administrator session expired"?reason:"Could not delete organization.");
  }finally{deleteBusy.current=false;setDeleting(false);}
 }
 useEffect(()=>{const q=new URLSearchParams(routeSearch);setSearch(q.get("search")||"");setStatus(q.get("status")||"");setPage(1);if(q.get("create")==="1")create();else setModal(false)},[routeSearch]);
 function create(){setEditing(null);setDraft(empty);setModal(true);setMessage("");}function edit(o:Org){setEditing(o);setDraft({name:o.name,email:o.email??"",status:o.status});setModal(true);setMessage("");}
 async function save(e:React.FormEvent){e.preventDefault();setSaving(true);setError("");try{await adminApi(editing?`/admin/organizations/${editing.id}`:"/admin/organizations",{method:editing?"PUT":"POST",body:JSON.stringify(draft)});setModal(false);setMessage(editing?"Organization updated.":"Organization created.");await load();}catch(e){setError(e instanceof Error?e.message:"Could not save organization");}finally{setSaving(false);}}
 async function view(o:Org){setDetailLoading(true);setDetail(null);setError("");try{const r=await adminApi(`/admin/organizations/${o.id}`);setDetail(r.data);}catch(e){setError(e instanceof Error?e.message:"Could not load organization");}finally{setDetailLoading(false);}}
 const pages=result?.pagination.pages??0;
 return <section className="max-w-7xl"><p className="text-sm text-slate-500">Admin management</p><div className="mt-1 flex flex-wrap items-center justify-between gap-3"><h1 className="text-3xl font-bold">Organizations</h1><button disabled={deleting} onClick={create} className="rounded-xl bg-blue-600 px-4 py-2.5 font-semibold text-white">Create organization</button></div><p className="mt-2 text-sm text-slate-500">Manage organizations and inspect users and cards associated through existing card ownership.</p>
 {message&&<div role="status" className="mt-5 rounded-xl border border-green-200 bg-green-50 p-3 text-green-800">{message}</div>}{error&&<div role="alert" className="mt-5 rounded-xl border border-red-200 bg-red-50 p-3 text-red-800">{error}<button type="button" disabled={deleting} onClick={()=>void load()} className="ml-3 underline disabled:opacity-50">Retry</button></div>}
 <div className="mt-6 flex flex-wrap gap-3 rounded-2xl border bg-white p-4"><input disabled={deleting} aria-label="Search organizations" value={search} onChange={e=>{setPage(1);setSearch(e.target.value);}} placeholder="Search name or email" className="min-w-64 flex-1 rounded-xl border px-4 py-2.5"/><select disabled={deleting} aria-label="Filter status" value={status} onChange={e=>{setPage(1);setStatus(e.target.value);}} className="rounded-xl border px-4 py-2.5"><option value="">All statuses</option><option value="active">Active</option><option value="inactive">Inactive</option></select></div>
 <div className="mt-4 overflow-hidden rounded-2xl border bg-white">{loading?<div className="p-10 text-center text-slate-500">Loading organizations…</div>:result?.data.length===0?<div className="p-10 text-center"><h2 className="font-semibold">No organizations found</h2><p className="mt-1 text-sm text-slate-500">0 organizations</p><p className="mt-1 text-sm text-slate-500">Try another search or create an organization.</p></div>:<><div className="overflow-x-auto"><table className="w-full text-left text-sm"><thead className="bg-slate-50 text-slate-500"><tr><th className="px-5 py-3 font-medium">Organization</th><th className="px-5 py-3 font-medium">Status</th><th className="px-5 py-3 font-medium">Users / cards</th><th className="px-5 py-3 font-medium">Created</th><th className="px-5 py-3 text-right font-medium">Actions</th></tr></thead><tbody className="divide-y">{result?.data.map(o=><tr key={o.id}><td className="px-5 py-4"><div className="font-semibold">{o.name}</div><div className="text-slate-500">{o.email||"No email"}</div></td><td className="px-5 py-4"><span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${o.status==="active"?"bg-green-50 text-green-700":"bg-slate-100 text-slate-600"}`}>{o.status}</span></td><td className="px-5 py-4 text-slate-600">{o.user_count} users · {o.card_count} cards</td><td className="px-5 py-4 text-slate-600">{new Date(o.created_at).toLocaleDateString()}</td><td className="px-5 py-4"><div className="flex justify-end gap-3"><button disabled={deleting} onClick={()=>void view(o)} className="font-medium text-blue-700">View</button><button disabled={deleting} onClick={()=>edit(o)} className="font-medium text-blue-700">Edit</button><button disabled={deleting||loading||saving} onClick={()=>confirmDelete(o)} className="font-medium text-red-700 disabled:opacity-50">Delete</button></div></td></tr>)}</tbody></table></div><div className="flex items-center justify-between border-t px-5 py-3 text-sm text-slate-600"><span>{result?.pagination.total??0} organizations</span><div className="flex items-center gap-3"><button disabled={deleting||page<=1} onClick={()=>setPage(n=>n-1)} className="rounded-lg border px-3 py-1.5 disabled:opacity-40">Previous</button><span>Page {page} of {Math.max(pages,1)}</span><button disabled={deleting||page>=pages} onClick={()=>setPage(n=>n+1)} className="rounded-lg border px-3 py-1.5 disabled:opacity-40">Next</button></div></div></>}</div>
 {modal&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4" role="dialog" aria-modal="true"><form onSubmit={save} className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl"><h2 className="text-xl font-bold">{editing?"Edit organization":"Create organization"}</h2><label className="mt-5 block text-sm font-medium">Name<input required maxLength={200} value={draft.name} onChange={e=>setDraft({...draft,name:e.target.value})} className="mt-1 w-full rounded-xl border px-3 py-2.5"/></label><label className="mt-4 block text-sm font-medium">Email<input type="email" maxLength={255} value={draft.email} onChange={e=>setDraft({...draft,email:e.target.value})} className="mt-1 w-full rounded-xl border px-3 py-2.5"/></label><label className="mt-4 block text-sm font-medium">Status<select value={draft.status} onChange={e=>setDraft({...draft,status:e.target.value})} className="mt-1 w-full rounded-xl border px-3 py-2.5"><option value="active">Active</option><option value="inactive">Inactive</option></select></label><div className="mt-6 flex justify-end gap-3"><button type="button" onClick={()=>setModal(false)} className="rounded-xl border px-4 py-2">Cancel</button><button disabled={saving} className="rounded-xl bg-blue-600 px-4 py-2 font-semibold text-white">{saving?"Saving…":editing?"Save changes":"Create organization"}</button></div></form></div>}
 {(detail||detailLoading)&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4" role="dialog" aria-modal="true"><div className="max-h-[90vh] w-full max-w-2xl overflow-auto rounded-2xl bg-white p-6 shadow-xl">{detailLoading?<p className="p-8 text-center text-slate-500">Loading organization…</p>:detail&&<><div className="flex items-start justify-between"><div><h2 className="text-xl font-bold">{detail.name}</h2><p className="text-sm text-slate-500">{detail.email||"No email"} · {detail.status}</p><p className="mt-1 text-xs text-slate-400">Created {new Date(detail.created_at).toLocaleString()}</p></div><button onClick={()=>setDetail(null)} className="rounded-lg border px-3 py-1.5">Close</button></div><h3 className="mt-6 font-semibold">Related users ({detail.users.length})</h3>{detail.users.length?<ul className="mt-2 divide-y">{detail.users.map(u=><li key={u.id} className="py-2 text-sm"><span className="font-medium">{u.name}</span><span className="ml-2 text-slate-500">{u.email} · {u.status}</span></li>)}</ul>:<p className="mt-2 text-sm text-slate-500">No linked users.</p>}<h3 className="mt-5 font-semibold">Cards ({detail.cards.length})</h3>{detail.cards.length?<ul className="mt-2 divide-y">{detail.cards.map(c=><li key={c.id} className="flex justify-between py-2 text-sm"><span><b>{c.name}</b><span className="ml-2 text-slate-500">/{c.slug}</span></span><span className="text-slate-500">{c.status}</span></li>)}</ul>:<p className="mt-2 text-sm text-slate-500">No linked cards.</p>}</>}</div></div>}
 {deleteTarget&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4" role="dialog" aria-modal="true" aria-labelledby="organization-delete-title" aria-describedby="organization-delete-description" onKeyDown={event=>{
  if(event.key==="Escape"&&!deleteBusy.current)setDeleteTarget(null);
  if(event.key==="Tab"){
   const buttons=Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>("button:not(:disabled)"));
   const first=buttons[0],last=buttons[buttons.length-1];
   if(event.shiftKey&&document.activeElement===first){event.preventDefault();last?.focus();}
   else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first?.focus();}
  }
 }}><div className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl">
  <h2 id="organization-delete-title" className="text-xl font-bold">Delete organization &apos;{deleteTarget.name}&apos;?</h2>
  <p id="organization-delete-description" className="mt-3 text-sm text-slate-600">This action will remove the organization. Organizations with associated records cannot be deleted. Continue?</p>
  {error&&<p role="alert" className="mt-3 text-sm text-red-700">{error}</p>}
  <div className="mt-6 flex justify-end gap-3"><button type="button" autoFocus disabled={deleting} onClick={()=>setDeleteTarget(null)} className="rounded-xl border px-4 py-2 disabled:opacity-50">Cancel</button><button type="button" disabled={deleting} onClick={removeOrganization} className="rounded-xl bg-red-600 px-4 py-2 font-semibold text-white disabled:opacity-50">{deleting?"Deleting…":"Delete"}</button></div>
 </div></div>}
 </section>;
}
