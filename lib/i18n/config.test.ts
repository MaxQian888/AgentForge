import {
  DEFAULT_LOCALE,
  isLocale,
  SUPPORTED_LOCALES,
} from "./config";

describe("i18n config", () => {
  it("exports the supported locales and default locale", () => {
    expect(SUPPORTED_LOCALES).toEqual(["zh-CN", "en"]);
    expect(DEFAULT_LOCALE).toBe("en");
  });

  it("recognizes valid locales and rejects unsupported values", () => {
    expect(isLocale("zh-CN")).toBe(true);
    expect(isLocale("en")).toBe(true);
    expect(isLocale("fr")).toBe(false);
    expect(isLocale(null)).toBe(false);
  });
});
