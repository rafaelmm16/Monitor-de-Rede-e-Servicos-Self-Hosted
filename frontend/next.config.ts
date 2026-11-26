import type { NextConfig } from "next";

const isPages = process.env.GITHUB_PAGES === "true";
const repoBase = "/Monitor-de-Rede-e-Servicos-Self-Hosted";

const backendTarget = (
  process.env.BACKEND_INTERNAL_URL ||
  process.env.NEXT_PUBLIC_API_URL ||
  "http://localhost:8080"
).replace(/\/$/, "");

const nextConfig: NextConfig = isPages
  ? {
      output: "export",
      basePath: repoBase,
      images: {
        unoptimized: true,
      },
    }
  : {
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
