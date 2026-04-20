"use client";

import Link from "next/link";
import { use } from "react";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

/**
 * Employee detail layout — owns the side-nav for per-employee sub-pages.
 * Spec 1A introduces the "Runs" tab; Specs 1C/1D will hang Triggers /
 * Secrets tabs off the same nav. Do NOT duplicate this file in those
 * downstream plans.
 */
interface NavTab {
  slug: string;
  label: string;
}

const TABS: NavTab[] = [
  { slug: "runs", label: "Runs" },
  // 1C will append: { slug: "triggers", label: "Triggers" }
  // 1D will append: { slug: "secrets",  label: "Secrets"  }
];

export default function EmployeeDetailLayout({
  params,
  children,
}: {
  params: Promise<{ id: string }>;
  children: React.ReactNode;
}) {
  const { id } = use(params);
  const pathname = usePathname();
  return (
    <div className="space-y-4">
      <div className="border-b">
        <nav className="flex gap-1 px-1" aria-label="Employee sections">
          {TABS.map((tab) => {
            const href = `/employees/${id}/${tab.slug}`;
            const active = pathname?.startsWith(href);
            return (
              <Link
                key={tab.slug}
                href={href}
                className={cn(
                  "px-4 py-2 text-sm border-b-2 -mb-px transition-colors",
                  active
                    ? "border-primary text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground",
                )}
              >
                {tab.label}
              </Link>
            );
          })}
        </nav>
      </div>
      {children}
    </div>
  );
}
