"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { adminApi } from "@/lib/admin-api";
import {useAdminLocation} from "@/lib/admin-location";

type User = { id:number; public_id:string; name:string; email:string; status:string; created_at:string; updated_at:string };
type UserPage = { data:User[]; pagination:{page:number;limit:number;total:number;pages:number} };
type Draft = { name:string; email:string; status:string };
const blank:Draft = { name:"", email:"", status:"active" };

export default function UsersPage() {
  const {search:routeSearch}=useAdminLocation();
  const [result,setResult]=useState<UserPage|null>(null);
  const [page,setPage]=useState(1); const [search,setSearch]=useState(""); const [status,setStatus]=useState("");
  const [loading,setLoading]=useState(true); const [error,setError]=useState(""); const [notice,setNotice]=useState("");
  const [editing,setEditing]=useState<User|null>(null); const [modalOpen,setModalOpen]=useState(false); const [viewing,setViewing]=useState<User|null>(null); const [draft,setDraft]=useState<Draft>(blank); const [saving,setSaving]=useState(false);
  const [deleteTarget,setDeleteTarget]=useState<User|null>(null),[deleting,setDeleting]=useState(false);
  const deleteBusy=useRef(false),listRequest=useRef(0);
  const load=useCallback(async()=>{
    const request=++listRequest.current;
    setLoading(true);setError("");
    try{
      let requestedPage=page;
      let next:UserPage;
      for(;;){
        const q=new URLSearchParams({page:String(requestedPage),limit:"20",search,status});
        next=await adminApi(`/admin/users?${q}`);
        if(request!==listRequest.current)return false;
        const lastPage=Math.max(next.pagination.pages,1);
        if(requestedPage<=lastPage)break;
        requestedPage=lastPage;
      }
      setResult(next);setPage(requestedPage);return true;
    }catch(e){if(request===listRequest.current)setError(e instanceof Error?e.message:"Could not load users");return false;}
    finally{if(request===listRequest.current)setLoading(false);}
  },[page,search,status]);
  const latestLoad=useRef(load);latestLoad.current=load;
  function confirmDelete(user:User){if(deleteBusy.current||loading||saving)return;setError("");setNotice("");setDeleteTarget(user);}
  async function removeUser(){
    if(!deleteTarget||deleteBusy.current)return;
    deleteBusy.current=true;listRequest.current+=1;setDeleting(true);setError("");setNotice("");
    try{
      await adminApi(`/admin/users/${deleteTarget.id}`,{method:"DELETE"});
      listRequest.current+=1;setDeleteTarget(null);setResult(null);
      setViewing(current=>current?.id===deleteTarget.id?null:current);
      setNotice("User deleted successfully.");
      if(!await latestLoad.current())setError("User was deleted, but the list could not be reloaded. Retry to load users.");
    }catch(e){
      const reason=e instanceof Error?e.message:"";
      setError(reason.startsWith("You cannot delete the user matching")||reason==="User not found"||reason==="User has already been deleted"||reason==="Permission denied"||reason==="Administrator session expired"?reason:"Could not delete user.");
    }finally{deleteBusy.current=false;setDeleting(false);}
  }
  useEffect(()=>{void load();},[load]);
  useEffect(()=>{const q=new URLSearchParams(routeSearch);setSearch(q.get("search")||"");setStatus(q.get("status")||"");setPage(1);if(q.get("create")==="1")startCreate();else setModalOpen(false)},[routeSearch]);
  function startCreate(){setEditing(null);setDraft(blank);setModalOpen(true);setNotice("");}
  function startEdit(user:User){setEditing(user);setDraft({name:user.name,email:user.email,status:user.status});setModalOpen(true);setNotice("");}
  async function submit(e:React.FormEvent){e.preventDefault();setSaving(true);setError("");setNotice("");try{await adminApi(editing?`/admin/users/${editing.id}`:"/admin/users",{method:editing?"PUT":"POST",body:JSON.stringify(draft)});setModalOpen(false);setEditing(null);setNotice(editing?"User updated.":"User created.");await load();}catch(e){setError(e instanceof Error?e.message:"Could not save user");}finally{setSaving(false);}}
  async function setUserStatus(user:User,next:string){setError("");setNotice("");try{await adminApi(`/admin/users/${user.id}`,{method:"PUT",body:JSON.stringify({name:user.name,email:user.email,status:next})});setNotice(`User ${next=== "active"?"activated":"suspended"}.`);await load();if(viewing?.id===user.id)setViewing({...user,status:next});}catch(e){setError(e instanceof Error?e.message:"Could not update status");}}
  const pages=result?.pagination.pages??0;
  return <section className="max-w-7xl">
    <p className="text-sm text-slate-500">Admin management</p><div className="mt-1 flex flex-wrap items-center justify-between gap-3"><h1 className="text-3xl font-bold">Users</h1><button disabled={deleting} onClick={startCreate} className="rounded-xl bg-blue-600 px-4 py-2.5 font-semibold text-white hover:bg-blue-700">Create user</button></div>
    <p className="mt-2 text-sm text-slate-500">Manage platform users. Account credentials and password hashes are never shown here.</p>
    {notice&&<div role="status" className="mt-5 rounded-xl border border-green-200 bg-green-50 p-3 text-green-800">{notice}</div>}{error&&<div role="alert" className="mt-5 rounded-xl border border-red-200 bg-red-50 p-3 text-red-800">{error}<button type="button" disabled={deleting} onClick={()=>void load()} className="ml-3 underline disabled:opacity-50">Retry</button></div>}
    <div className="mt-6 flex flex-wrap gap-3 rounded-2xl border bg-white p-4"><input disabled={deleting} aria-label="Search users" value={search} onChange={e=>{setPage(1);setSearch(e.target.value);}} placeholder="Search name or email" className="min-w-64 flex-1 rounded-xl border px-4 py-2.5"/><select disabled={deleting} aria-label="Filter status" value={status} onChange={e=>{setPage(1);setStatus(e.target.value);}} className="rounded-xl border px-4 py-2.5"><option value="">All statuses</option><option value="active">Active</option><option value="inactive">Inactive</option><option value="suspended">Suspended</option></select></div>
    <div className="mt-4 overflow-hidden rounded-2xl border bg-white">{loading?<div className="p-10 text-center text-slate-500">Loading users…</div>:result?.data.length===0?<div className="p-10 text-center"><h2 className="font-semibold">No users found</h2><p className="mt-1 text-sm text-slate-500">0 users</p><p className="mt-1 text-sm text-slate-500">Try changing your search or create a user.</p></div>:<><div className="overflow-x-auto"><table className="w-full text-left text-sm"><thead className="bg-slate-50 text-slate-500"><tr><th className="px-5 py-3 font-medium">User</th><th className="px-5 py-3 font-medium">Status</th><th className="px-5 py-3 font-medium">Created</th><th className="px-5 py-3 text-right font-medium">Actions</th></tr></thead><tbody className="divide-y">{result?.data.map(user=><tr key={user.id}><td className="px-5 py-4"><div className="font-semibold text-slate-900">{user.name}</div><div className="text-slate-500">{user.email}</div></td><td className="px-5 py-4"><span className={`rounded-full px-2.5 py-1 text-xs font-semibold ${user.status==="active"?"bg-green-50 text-green-700":user.status==="suspended"?"bg-red-50 text-red-700":"bg-slate-100 text-slate-600"}`}>{user.status}</span></td><td className="px-5 py-4 text-slate-600">{new Date(user.created_at).toLocaleDateString()}</td><td className="px-5 py-4"><div className="flex justify-end gap-3"><button disabled={deleting} className="font-medium text-blue-700" onClick={()=>setViewing(user)}>View</button><button disabled={deleting} className="font-medium text-blue-700" onClick={()=>startEdit(user)}>Edit</button>{user.status==="active"?<button disabled={deleting} className="font-medium text-red-700" onClick={()=>void setUserStatus(user,"suspended")}>Suspend</button>:<button disabled={deleting} className="font-medium text-green-700" onClick={()=>void setUserStatus(user,"active")}>Activate</button>}<button disabled={deleting||loading||saving} className="font-medium text-red-700 disabled:opacity-50" onClick={()=>confirmDelete(user)}>Delete</button></div></td></tr>)}</tbody></table></div><div className="flex items-center justify-between border-t px-5 py-3 text-sm text-slate-600"><span>{result?.pagination.total??0} users</span><div className="flex items-center gap-3"><button disabled={deleting||page<=1} onClick={()=>setPage(v=>v-1)} className="rounded-lg border px-3 py-1.5 disabled:opacity-40">Previous</button><span>Page {page} of {Math.max(1,pages)}</span><button disabled={deleting||page>=pages} onClick={()=>setPage(v=>v+1)} className="rounded-lg border px-3 py-1.5 disabled:opacity-40">Next</button></div></div></>}</div>
    {modalOpen&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4" role="dialog" aria-modal="true"><form onSubmit={submit} className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl"><h2 className="text-xl font-bold">{editing?"Edit user":"Create user"}</h2><label className="mt-5 block text-sm font-medium">Name<input required maxLength={150} value={draft.name} onChange={e=>setDraft({...draft,name:e.target.value})} className="mt-1 w-full rounded-xl border px-3 py-2.5"/></label><label className="mt-4 block text-sm font-medium">Email<input required type="email" maxLength={255} value={draft.email} onChange={e=>setDraft({...draft,email:e.target.value})} className="mt-1 w-full rounded-xl border px-3 py-2.5"/></label><label className="mt-4 block text-sm font-medium">Status<select value={draft.status} onChange={e=>setDraft({...draft,status:e.target.value})} className="mt-1 w-full rounded-xl border px-3 py-2.5"><option value="active">Active</option><option value="inactive">Inactive</option><option value="suspended">Suspended</option></select></label><div className="mt-6 flex justify-end gap-3"><button type="button" onClick={()=>{setModalOpen(false);setEditing(null);setDraft(blank);}} className="rounded-xl border px-4 py-2">Cancel</button><button disabled={saving} className="rounded-xl bg-blue-600 px-4 py-2 font-semibold text-white disabled:opacity-50">{saving?"Saving…":editing?"Save changes":"Create user"}</button></div></form></div>}
    {viewing&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4" role="dialog" aria-modal="true"><div className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl"><h2 className="text-xl font-bold">User details</h2><dl className="mt-5 space-y-4 text-sm"><div><dt className="text-slate-500">Name</dt><dd className="font-medium">{viewing.name}</dd></div><div><dt className="text-slate-500">Email</dt><dd className="font-medium">{viewing.email}</dd></div><div><dt className="text-slate-500">Status</dt><dd className="font-medium capitalize">{viewing.status}</dd></div><div><dt className="text-slate-500">Created</dt><dd className="font-medium">{new Date(viewing.created_at).toLocaleString()}</dd></div><div><dt className="text-slate-500">User ID</dt><dd className="font-mono">{viewing.public_id}</dd></div></dl><div className="mt-6 flex justify-end"><button onClick={()=>setViewing(null)} className="rounded-xl bg-slate-900 px-4 py-2 font-semibold text-white">Close</button></div></div></div>}
    {deleteTarget&&<div className="fixed inset-0 z-50 grid place-items-center bg-slate-900/40 p-4" role="dialog" aria-modal="true" aria-labelledby="user-delete-title" aria-describedby="user-delete-description" onKeyDown={event=>{
      if(event.key==="Escape"&&!deleteBusy.current)setDeleteTarget(null);
      if(event.key==="Tab"){
        const buttons=Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>("button:not(:disabled)"));
        const first=buttons[0],last=buttons[buttons.length-1];
        if(event.shiftKey&&document.activeElement===first){event.preventDefault();last?.focus();}
        else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first?.focus();}
      }
    }}><div className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl">
      <h2 id="user-delete-title" className="text-xl font-bold">Delete user &apos;{deleteTarget.name}&apos;?</h2>
      <p id="user-delete-description" className="mt-3 text-sm text-slate-600">This user will be removed from active use and their account details anonymized. Login access will be disabled, public cards archived, and active subscriptions cancelled. Cards and historical data will be preserved.</p>
      {error&&<p role="alert" className="mt-3 text-sm text-red-700">{error}</p>}
      <div className="mt-6 flex justify-end gap-3"><button type="button" autoFocus disabled={deleting} onClick={()=>setDeleteTarget(null)} className="rounded-xl border px-4 py-2 disabled:opacity-50">Cancel</button><button type="button" disabled={deleting} onClick={removeUser} className="rounded-xl bg-red-600 px-4 py-2 font-semibold text-white disabled:opacity-50">{deleting?"Deleting…":"Delete"}</button></div>
    </div></div>}
  </section>;
}
