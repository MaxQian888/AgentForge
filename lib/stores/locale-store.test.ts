import {
  DEFAULT_LOCALE,
  getPreferredLocale,
  LOCALE_STORAGE_KEY,
  useLocaleStore,
} from "./locale-store";

describe("useLocaleStore", () => {
  beforeEach(() => {
    window.localStorage.clear();
    useLocaleStore.setState({ locale: DEFAULT_LOCALE });
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("defaults to the shared default locale and allows updates", () => {
    expect(useLocaleStore.getState().locale).toBe(DEFAULT_LOCALE);

    useLocaleStore.getState().setLocale("zh-CN");

    expect(useLocaleStore.getState().locale).toBe("zh-CN");
  });

  it("returns the in-memory locale once persistence has hydrated", () => {
    jest.spyOn(useLocaleStore.persist, "hasHydrated").mockReturnValue(true);
    useLocaleStore.setState({ locale: "zh-CN" });

    expect(getPreferredLocale()).toBe("zh-CN");
  });

  it("reads a valid persisted locale before hydration finishes", () => {
    jest.spyOn(useLocaleStore.persist, "hasHydrated").mockReturnValue(false);
    window.localStorage.setItem(
      LOCALE_STORAGE_KEY,
      JSON.stringify({ state: { locale: "zh-CN" } }),
    );

    expect(getPreferredLocale()).toBe("zh-CN");
  });

  it("falls back to the current locale when persisted data is invalid", () => {
    jest.spyOn(useLocaleStore.persist, "hasHydrated").mockReturnValue(false);
    useLocaleStore.setState({ locale: "en" });

    window.localStorage.setItem(
      LOCALE_STORAGE_KEY,
      JSON.stringify({ state: { locale: "fr" } }),
    );
    expect(getPreferredLocale()).toBe("en");

    window.localStorage.setItem(LOCALE_STORAGE_KEY, "{not-json");
    expect(getPreferredLocale()).toBe("en");
  });
});
