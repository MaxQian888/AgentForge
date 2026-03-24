jest.mock("next/font/google", () => ({
  Geist: () => ({ variable: "--font-geist-sans" }),
  Geist_Mono: () => ({ variable: "--font-geist-mono" }),
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

  it("renders html/body with font variables and children", () => {
    const markup = renderToStaticMarkup(
      <RootLayout>
        <main>content</main>
      </RootLayout>
    );

    expect(markup).toContain('<html lang="en">');
    expect(markup).toContain("--font-geist-sans");
    expect(markup).toContain("--font-geist-mono");
    expect(markup).toContain("antialiased");
    expect(markup).toContain("<main>content</main>");
  });
});
