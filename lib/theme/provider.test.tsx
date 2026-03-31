import { act, render } from "@testing-library/react";
import { useEffect } from "react";
import { ThemeProvider } from "./provider";
import { useTheme } from "./provider";

describe("ThemeProvider", () => {
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

  it("removes the same media-query listener when switching away from system theme", () => {
    const mediaQuery = {
      matches: false,
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
    } as unknown as MediaQueryList;
    const matchMediaSpy = jest
      .spyOn(window, "matchMedia")
      .mockReturnValue(mediaQuery);

    const themeController: { current?: (theme: string) => void } = {};

    function ThemeConsumer() {
      const context = useTheme();

      useEffect(() => {
        themeController.current = context.setTheme;
      }, [context.setTheme]);

      return null;
    }

    render(
      <ThemeProvider
        attribute="class"
        defaultTheme="system"
        enableSystem
        disableTransitionOnChange
      >
        <ThemeConsumer />
      </ThemeProvider>,
    );

    expect(mediaQuery.addEventListener).toHaveBeenCalledTimes(1);
    const subscribedListener = (mediaQuery.addEventListener as jest.Mock).mock.calls[0]?.[1];

    act(() => {
      themeController.current?.("light");
    });

    expect(mediaQuery.removeEventListener).toHaveBeenCalledTimes(1);
    expect((mediaQuery.removeEventListener as jest.Mock).mock.calls[0]?.[1]).toBe(
      subscribedListener,
    );

    matchMediaSpy.mockRestore();
  });
});
