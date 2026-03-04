import { DatabaseZap, Gauge, Layers3, ShieldCheck } from "lucide-react";
import { NavLink, Outlet } from "react-router";

import { Badge } from "../components/ui/badge";
import { Separator } from "../components/ui/separator";
import { cn } from "../lib/utils";

const navItems = [
  { to: "/deployments", label: "Deployments", icon: Layers3 },
  { to: "/system", label: "System", icon: Gauge },
];

export function DashboardLayout() {
  return (
    <div className="relative min-h-screen overflow-hidden bg-[var(--canvas)] text-[var(--ink)]">
      <div className="pointer-events-none absolute -top-56 left-[-14rem] h-[34rem] w-[34rem] rounded-full bg-[radial-gradient(circle,_rgba(64,137,255,0.28)_0%,_rgba(64,137,255,0)_68%)]" />
      <div className="pointer-events-none absolute -bottom-72 right-[-10rem] h-[34rem] w-[34rem] rounded-full bg-[radial-gradient(circle,_rgba(255,157,80,0.27)_0%,_rgba(255,157,80,0)_72%)]" />

      <div className="relative mx-auto grid min-h-screen max-w-[1240px] grid-cols-1 gap-5 p-4 md:grid-cols-[260px_1fr] md:p-6">
        <aside className="rounded-3xl border border-[var(--line)] bg-[var(--panel)]/95 p-5 backdrop-blur">
          <div className="mb-6 flex items-center gap-3">
            <div className="grid h-10 w-10 place-content-center rounded-xl bg-[var(--accent)] text-[var(--accent-foreground)]">
              <DatabaseZap size={18} />
            </div>
            <div>
              <div className="font-title text-lg leading-none">Warden</div>
              <div className="mt-1 text-xs text-[var(--muted)]">Refine Dashboard</div>
            </div>
          </div>

          <Badge variant="success" className="mb-5 w-fit">
            <ShieldCheck size={12} className="mr-1" />
            Node healthy
          </Badge>
          <Separator />

          <nav className="mt-4 grid gap-2">
            {navItems.map((item) => {
              const Icon = item.icon;
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={({ isActive }) =>
                    cn(
                      "flex items-center gap-2 rounded-xl px-3 py-2 text-sm transition",
                      isActive
                        ? "bg-[var(--accent)] text-[var(--accent-foreground)] shadow-[0_12px_30px_-14px_rgba(48,111,236,0.95)]"
                        : "text-[var(--panel-foreground)] hover:bg-[var(--panel-muted)]",
                    )
                  }
                >
                  <Icon size={16} />
                  {item.label}
                </NavLink>
              );
            })}
          </nav>
        </aside>

        <main className="rounded-3xl border border-[var(--line)] bg-[var(--panel)]/90 p-5 backdrop-blur md:p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
