"use client";

import { usePathname } from "next/navigation";

import AdminHeader from "@/components/admin/AdminHeader";
import AdminSidebar from "@/components/admin/AdminSidebar";
import AdminSessionGuard from "@/components/admin/AdminSessionGuard";
import { AdminLocaleProvider } from "@/lib/admin-locale";
import { useState } from "react";

const publicAdminRoutes = [
  "/admin/login",
  "/admin/forgot-password",
  "/admin/verify-otp",
  "/admin/reset-password",
];

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();

  const isPublicAdminRoute = publicAdminRoutes.some(
    (route) =>
      pathname === route ||
      pathname.startsWith(`${route}/`)
  );

  // Authentication pages must NOT render the admin dashboard shell.
  if (isPublicAdminRoute) {
    return <>{children}</>;
  }

  // Everything else under /admin is protected.
  return (
    <AdminSessionGuard>
      <AdminLocaleProvider>
        <AdminShell>{children}</AdminShell>
      </AdminLocaleProvider>
    </AdminSessionGuard>
  );
}

function AdminShell({ children }: { children: React.ReactNode }) {
  const [menuOpen, setMenuOpen] = useState(false);
  return (
        <div className="flex min-h-screen min-w-0 bg-slate-50">
          <AdminSidebar open={menuOpen} onClose={() => setMenuOpen(false)} />

          <div className="min-w-0 flex-1">
            <AdminHeader onMenu={() => setMenuOpen(true)} menuOpen={menuOpen} />

            <main className="min-w-0 p-4 sm:p-6 lg:p-8">
              {children}
            </main>
          </div>
        </div>
  );
}
