import type { NextConfig } from "next";

const backendTarget = (
  process.env.BACKEND_INTERNAL_URL ||
  process.env.NEXT_PUBLIC_API_URL ||
  "http://localhost:8080"
).replace(/\/$/, "");

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${backendTarget}/api/:path*`,
      },
      {
        source: "/metrics",
        destination: `${backendTarget}/metrics`,
      },
    ];
  },
};

export default nextConfig;
