jest.mock("@/lib/theme/provider", () => ({
  ThemeProvider: ({ children }: { children: React.ReactNode }) => children,
}));

jest.mock("@/lib/i18n/provider", () => ({
  I18nProvider: ({ children }: { children: React.ReactNode }) => children,
}));

jest.mock("@/lib/stores/locale-store", () => ({
  DEFAULT_LOCALE: "en",
}));

jest.mock("@/components/ui/tooltip", () => ({
  TooltipProvider: ({ children }: { children: React.ReactNode }) => children,
}));

import { renderToStaticMarkup } from "react-dom/server";
import RootLayout, { metadata } from "./layout";

describe("RootLayout", () => {
  it("exports metadata used by Next.js", () => {
    expect(metadata).toMatchObject({
      title: "AgentForge",
      description: "AI Agent Orchestration Platform",
    });
  });

  it("renders html/body and children", () => {
    const markup = renderToStaticMarkup(
      <RootLayout>
        <main>content</main>
      </RootLayout>
    );

    expect(markup).toContain('<html lang="en"');
    expect(markup).toContain("font-sans");
    expect(markup).toContain("antialiased");
    expect(markup).toContain('data-slot="desktop-window-frame"');
    expect(markup).toContain('data-slot="desktop-window-content"');
    expect(markup).toContain("<main>content</main>");
  });
});
