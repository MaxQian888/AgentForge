import type { Metadata } from "next";
import { NextIntlClientProvider } from "next-intl";
import { DesktopWindowFrame } from "@/components/layout/desktop-window-frame";
import { TooltipProvider } from "@/components/ui/tooltip";
import { I18nProvider } from "@/lib/i18n/provider";
import { DEFAULT_LOCALE } from "@/lib/i18n/config";
import { ThemeProvider } from "@/lib/theme/provider";
import "./globals.css";

export const metadata: Metadata = {
  title: "AgentForge",
  description: "AI Agent Orchestration Platform",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang={DEFAULT_LOCALE} suppressHydrationWarning>
      <body className="font-sans antialiased">
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
          <NextIntlClientProvider>
            <I18nProvider initialLocale={DEFAULT_LOCALE}>
              <TooltipProvider>
                <DesktopWindowFrame>{children}</DesktopWindowFrame>
              </TooltipProvider>
            </I18nProvider>
          </NextIntlClientProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
