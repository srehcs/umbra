import Link from "next/link";
import { Shield, Activity, Wrench, FileText } from "lucide-react";
import { cn } from "@/lib/utils";
import { Separator } from "@/components/ui/separator";
import TenantSwitcher from "@/components/app/tenant-switcher";

const nav = [
  { href: "/receipts", label: "Receipts", icon: FileText },
  { href: "/tools", label: "Tools", icon: Wrench },
  { href: "/policies", label: "Policies", icon: Shield },
];

export default function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen bg-white">
      <div className="flex">
        <aside className="hidden md:flex md:w-72 md:flex-col md:border-r md:border-border md:bg-white">
          <div className="flex items-center gap-2 px-6 py-6">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-white shadow-soft">
              <Shield className="h-5 w-5" />
            </div>
            <div className="leading-tight">
              <div className="text-sm font-semibold">Umbra V0-C</div>
              <div className="text-xs text-muted-foreground">Agent Identity Control Plane</div>
            </div>
          </div>
          <div className="px-6">
            <TenantSwitcher />
          </div>
          <Separator className="my-6" />
          <nav className="flex flex-col gap-1 px-3">
            {nav.map((item) => {
              const Icon = item.icon;
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={cn(
                    "flex items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-muted",
                  )}
                >
                  <Icon className="h-4 w-4" />
                  {item.label}
                </Link>
              );
            })}
          </nav>
          <div className="mt-auto px-6 py-6 text-xs text-muted-foreground">
            <div className="flex items-center gap-2">
              <Activity className="h-4 w-4" />
              Traces:{" "}
              <a href="http://localhost:16686" target="_blank" rel="noreferrer">
                Jaeger
              </a>
            </div>
          </div>
        </aside>

        <main className="flex-1">
          <header className="sticky top-0 z-10 border-b border-border bg-white/80 backdrop-blur">
            <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
              <div className="md:hidden">
                <div className="text-sm font-semibold">Umbra V0-C</div>
                <div className="text-xs text-muted-foreground">Console</div>
              </div>
              <div className="hidden md:flex items-center gap-2 text-sm">
                <span className="text-muted-foreground">Enterprise demo console</span>
                <span className="rounded-full border border-border bg-muted px-2 py-0.5 text-xs">Dev mode</span>
                <span className="rounded-full border border-border bg-white px-2 py-0.5 text-xs">Role: developer</span>
              </div>
              <div className="md:hidden">
                <TenantSwitcher compact />
              </div>
            </div>
          </header>

          <div className="mx-auto max-w-5xl px-6 py-8">{children}</div>
        </main>
      </div>
    </div>
  );
}
