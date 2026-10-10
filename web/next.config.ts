import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Allow the configured phone/LAN origin to connect to Next dev resources
  // (including the HMR WebSocket); this has no effect on production builds.
  allowedDevOrigins: ["192.168.29.31"],
  output: "standalone",
  async redirects() {
    return [
      { source: "/about", destination: "/contact#about-us", permanent: true },
      { source: "/faq", destination: "/contact#faq", permanent: true },
    ];
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "X-Frame-Options", value: "SAMEORIGIN" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
        ],
      },
    ];
  },
};

export default nextConfig;
