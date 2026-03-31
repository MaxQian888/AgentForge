/** @jest-environment node */

import { renderToStaticMarkup } from "react-dom/server";
import { ThemeProvider } from "./provider";

describe("ThemeProvider SSR", () => {
  it("includes the theme boot script in SSR markup", () => {
    const markup = renderToStaticMarkup(
      <ThemeProvider
        attribute="class"
        defaultTheme="system"
        enableSystem
        disableTransitionOnChange
      >
        <main>content</main>
      </ThemeProvider>,
    );

    expect(markup).toContain("<script");
    expect(markup).toContain("<main>content</main>");
  });
});
