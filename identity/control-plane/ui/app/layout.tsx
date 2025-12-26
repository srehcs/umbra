import "./globals.css";
import type { Metadata } from "next";
import AppShell from "@/components/app/app-shell";

export const metadata: Metadata = {
  title: "Umbra V0-C Console",
  description: "Agent Identity Control Plane (V0-C) demo console",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <AppShell>{children}</AppShell>
      </body>
    </html>
  );
}
