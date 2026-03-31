import { render } from "@testing-library/react";
import { renderToStaticMarkup } from "react-dom/server";
import { ThemeProvider } from "./provider";

describe("ThemeProvider", () => {
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

  it("does not render a script element on the client", () => {
    const { container } = render(
      <ThemeProvider
        attribute="class"
        defaultTheme="system"
        enableSystem
        disableTransitionOnChange
      >
        <main>content</main>
      </ThemeProvider>,
    );

    expect(container.querySelector("script")).toBeNull();
    expect(container.textContent).toContain("content");
  });
});
