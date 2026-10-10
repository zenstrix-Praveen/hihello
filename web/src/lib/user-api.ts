import { getApiBaseUrl } from "@/lib/api-config";
const API_URL = getApiBaseUrl();

export async function apiRequest(
  path: string,
  options: RequestInit = {}
) {
  const token =
    typeof window !== "undefined"
      ? localStorage.getItem("user_token")
      : null;

  const headers = new Headers(options.headers);

  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers,
  });

  const data = await response.json().catch(() => null);

  if (!response.ok) {
    if (
      response.status === 401 &&
      typeof window !== "undefined" &&
      (path.startsWith("/user/") || path.startsWith("/billing/"))
    ) {
      // A request made with an older token can finish after a fresh login.
      // Only invalidate the session that was actually sent with this request.
      if (getUserToken() === token) {
        localStorage.removeItem("user_token");
        window.dispatchEvent(new Event("customer:unauthorized"));
      }
    }
    throw new Error(
      data?.error ||
      data?.message ||
      `Request failed (${response.status})`
    );
  }

  return data;
}

export function saveUserToken(token: string) {
  localStorage.setItem("user_token", token);
}

export function getUserToken() {
  return localStorage.getItem("user_token");
}

export function logoutUser() {
  localStorage.removeItem("user_token");
  window.dispatchEvent(new Event("customer:logout"));
}
