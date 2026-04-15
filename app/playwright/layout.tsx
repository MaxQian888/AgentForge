import Link from "next/link"
import { notFound } from "next/navigation"
import { isPlaywrightHarnessEnabled } from "@/lib/playwright-harness"

export default function PlaywrightLayout({
  children,
}: {
  children: React.ReactNode
}) {
  if (!isPlaywrightHarnessEnabled()) {
    notFound()
  }

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-6xl flex-col gap-6 px-6 py-10">
      <header className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border/60 bg-card/70 px-4 py-3">
        <div>
          <h1 className="text-xl font-semibold">Playwright Harness</h1>
          <p className="text-sm text-muted-foreground">
            Internal browser-test routes for template management coverage.
          </p>
        </div>
        <nav className="flex gap-3 text-sm">
          <Link href="/playwright/docs-template" className="hover:text-primary">
            Docs Templates
          </Link>
          <Link href="/playwright/workflow-template" className="hover:text-primary">
            Workflow Templates
          </Link>
        </nav>
      </header>
      {children}
    </main>
  )
}
