"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { adminApi } from "@/lib/admin-api";

const publicAdminRoutes = [
  "/admin/login",
  "/admin/forgot-password",
  "/admin/verify-otp",
  "/admin/reset-password",
];

export default function AdminSessionGuard({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();

  const [checking, setChecking] = useState(true);
  const [authenticated, setAuthenticated] = useState(false);

  const isPublicRoute = publicAdminRoutes.includes(pathname);

  useEffect(() => {
    if (isPublicRoute) {
      setChecking(false);
      setAuthenticated(false);
      return;
    }

    let mounted = true;
    const handleUnauthorized = () => {
      if (!mounted) return;
      setAuthenticated(false);
      setChecking(false);
      router.replace("/admin/login");
    };
    window.addEventListener("admin:unauthorized", handleUnauthorized);

    async function validateAdmin() {
      const token = localStorage.getItem("admin_token");

      if (!token) {
        if (mounted) {
          setChecking(false);
          router.replace("/admin/login");
        }
        return;
      }

      try {
        await adminApi("/admin/auth/me");

        if (mounted) {
          setAuthenticated(true);
          setChecking(false);
        }
      } catch {
        localStorage.removeItem("admin_token");
        localStorage.removeItem("admin_user");

        if (mounted) {
          setAuthenticated(false);
          setChecking(false);
          router.replace("/admin/login");
        }
      }
    }

    void validateAdmin();

    return () => {
      mounted = false;
      window.removeEventListener("admin:unauthorized", handleUnauthorized);
    };
  }, [isPublicRoute, pathname, router]);

  if (isPublicRoute) {
    return <>{children}</>;
  }

  if (checking) {
    return (
      <main className="grid min-h-screen place-items-center bg-slate-50">
        <p className="text-slate-500">
          Checking administrator session...
        </p>
      </main>
    );
  }

  if (!authenticated) {
    return null;
  }

  return <>{children}</>;
}
