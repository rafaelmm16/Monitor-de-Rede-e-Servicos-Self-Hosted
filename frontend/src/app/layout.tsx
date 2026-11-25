import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "NetPulse — Self-Hosted Network & Service Monitor",
  description:
    "Real-time uptime, latency, TCP, ICMP, and SSL certificate monitoring powered by Go and Next.js.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body className="min-h-screen bg-slate-950 text-slate-100 antialiased selection:bg-emerald-500/30 selection:text-emerald-200">
        {children}
      </body>
    </html>
  );
}
