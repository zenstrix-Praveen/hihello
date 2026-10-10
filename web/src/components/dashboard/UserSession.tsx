"use client";
import { createContext, useContext } from "react";
export type UserProfile = { id: number; public_id: string; name: string; email: string; phone: string | null; status: string; created_at: string };
export const UserContext = createContext<UserProfile | null>(null);
export function useUser() {
  const user = useContext(UserContext);
  if (!user) throw new Error("User session is unavailable");
  return user;
}
