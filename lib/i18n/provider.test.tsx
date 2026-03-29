import React from "react";
import { act, render, screen } from "@testing-library/react";
import { renderToStaticMarkup } from "react-dom/server";
import { I18nProvider } from "./provider";

const nextIntlProviderMock = jest.fn(
  ({
    children,
    locale,
  }: {
    children: React.ReactNode;
    locale: string;
  }) => (
    <div data-testid="intl-provider" data-locale={locale}>
      {children}
    </div>
  )
);

type Locale = "zh-CN" | "en";

jest.mock("next-intl", () => ({
  NextIntlClientProvider: (props: { children: React.ReactNode; locale: string }) =>
    nextIntlProviderMock(props),
}));

jest.mock("@/lib/stores/locale-store", () => {
  const localeState: { locale: Locale } = { locale: "en" };
  const hasHydratedMock = jest.fn(() => true);
  let finishHydrationHandler: (() => void) | null = null;

  const useLocaleStore = Object.assign(
    (selector: (state: { locale: Locale }) => unknown) => selector(localeState),
    {
      persist: {
        hasHydrated: hasHydratedMock,
        onFinishHydration: jest.fn((callback: () => void) => {
          finishHydrationHandler = callback;
          return () => {
            finishHydrationHandler = null;
          };
        }),
      },
    }
  );

  return {
    DEFAULT_LOCALE: "en",
    useLocaleStore,
    __mockControls: {
      localeState,
      hasHydratedMock,
      triggerHydration: () => {
        hasHydratedMock.mockReturnValue(true);
        finishHydrationHandler?.();
      },
      resetHydrationHandler: () => {
        finishHydrationHandler = null;
      },
    },
  };
});

const localeStoreModule = jest.requireMock("@/lib/stores/locale-store") as {
  __mockControls: {
    localeState: { locale: Locale };
    hasHydratedMock: jest.Mock<boolean, []>;
    triggerHydration: () => void;
    resetHydrationHandler: () => void;
  };
};

describe("I18nProvider", () => {
  beforeEach(() => {
    localeStoreModule.__mockControls.localeState.locale = "en";
    localeStoreModule.__mockControls.hasHydratedMock.mockReset().mockReturnValue(true);
    nextIntlProviderMock.mockClear();
    localeStoreModule.__mockControls.resetHydrationHandler();
    document.documentElement.lang = "";
  });

  it("uses the provided server locale during the initial render", () => {
    const markup = renderToStaticMarkup(
      <I18nProvider initialLocale="en">
        <main>server content</main>
      </I18nProvider>
    );

    expect(markup).toContain('data-locale="en"');
    expect(markup).toContain("<main>server content</main>");
  });

  it("keeps the server locale until persistence hydration finishes", async () => {
    localeStoreModule.__mockControls.localeState.locale = "en";
    localeStoreModule.__mockControls.hasHydratedMock.mockReturnValue(false);

    render(
      <I18nProvider initialLocale="en">
        <main>localized content</main>
      </I18nProvider>
    );

    expect(screen.getByTestId("intl-provider")).toHaveAttribute("data-locale", "en");
    expect(document.documentElement.lang).toBe("en");

    await act(async () => {
      localeStoreModule.__mockControls.triggerHydration();
    });

    expect(screen.getByTestId("intl-provider")).toHaveAttribute("data-locale", "en");
    expect(document.documentElement.lang).toBe("en");
  });
});
