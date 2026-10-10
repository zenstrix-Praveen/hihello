import { getApiBaseUrl } from "@/lib/api-config";
const API_URL = getApiBaseUrl();

export async function adminApi(
  path: string,
  options: RequestInit = {}
) {
  const token =
    typeof window !== "undefined"
      ? localStorage.getItem("admin_token")
      : null;

  let res: Response;

  try {
    res = await fetch(`${API_URL}${path}`, {
      ...options,
      cache: options.cache || "no-store",
      headers: {
        "Content-Type": "application/json",
        ...(token
          ? {
              Authorization: `Bearer ${token}`,
            }
          : {}),
        ...options.headers,
      },
    });
  } catch {
    throw new Error(
      "Unable to reach the admin API. Check the connection and try again."
    );
  }

  let data: any = null;

  if (res.status !== 204) {
    const text = await res.text();

    if (text) {
      try {
        data = JSON.parse(text);
      } catch {
        data = { error: text };
      }
    }
  }

  if (res.status === 401) {
    if (typeof window !== "undefined") {
      localStorage.removeItem("admin_token");
      localStorage.removeItem("admin_user");
      window.dispatchEvent(new Event("admin:unauthorized"));
    }

    throw new Error(
      data?.error ||
        data?.message ||
        "Administrator session expired"
    );
  }

  if (!res.ok) {
    if (res.status === 403) {
      throw new Error(
        data?.error ||
          data?.message ||
          "You do not have permission to perform this action."
      );
    }

    if (res.status === 404) {
      throw new Error(
        data?.error ||
          data?.message ||
          "The requested record or page was not found."
      );
    }

    if (res.status === 400 || res.status === 422) {
      throw new Error(
        data?.error ||
          data?.message ||
          "Please check the submitted values."
      );
    }

    throw new Error(
      data?.error ||
        data?.message ||
        `Admin API request failed (${res.status}).`
    );
  }

  return data;
}
