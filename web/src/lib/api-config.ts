/**
 * API endpoint selection shared by browser and server requests.
 * Configure NEXT_PUBLIC_API_URL for deployed environments. During local
 * development, a browser opened from a LAN hostname should reach the API on
 * that same host instead of trying to reach localhost on the mobile device.
 */
export function getApiBaseUrl(): string {
  const configured = process.env.NEXT_PUBLIC_API_URL?.trim();
  if (process.env.NODE_ENV === "development" && typeof window !== "undefined") {
    const pageHost = window.location.hostname;
    const isLanPage = !["localhost", "127.0.0.1", "::1"].includes(pageHost);
    const pointsAtLocalhost = !configured || /^(https?:\/\/)?(localhost|127\.0\.0\.1|\[::1\])(?::\d+)?(?:\/|$)/i.test(configured);
    if (isLanPage && pointsAtLocalhost) {
      return `${window.location.protocol}//${pageHost}:8080/api/v1`;
    }
    if (configured) return configured.replace(/\/+$/, "");
    return `${window.location.protocol}//${window.location.hostname}:8080/api/v1`;
  }

  if (configured) return configured.replace(/\/+$/, "");
  return "http://localhost:8080/api/v1";
}
