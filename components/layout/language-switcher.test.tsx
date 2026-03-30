import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LanguageSwitcher } from "./language-switcher";

const setLocaleMock = jest.fn();
const localeState = {
  locale: "zh-CN" as "zh-CN" | "en",
  setLocale: setLocaleMock,
};

jest.mock("@/lib/stores/locale-store", () => ({
  SUPPORTED_LOCALES: ["zh-CN", "en"],
  useLocaleStore: (
    selector?: (state: typeof localeState) => unknown,
  ) => (selector ? selector(localeState) : localeState),
}));

describe("LanguageSwitcher", () => {
  beforeEach(() => {
    setLocaleMock.mockReset();
    localeState.locale = "zh-CN";
  });

  it("lists supported locales and switches to English", async () => {
    const user = userEvent.setup();

    render(<LanguageSwitcher />);

    await user.click(screen.getByRole("button"));

    const menuItems = await screen.findAllByRole("menuitem");
    expect(menuItems).toHaveLength(2);

    await user.click(screen.getByRole("menuitem", { name: "English" }));

    expect(setLocaleMock).toHaveBeenCalledWith("en");
  });

  it("highlights the active locale", async () => {
    const user = userEvent.setup();
    localeState.locale = "en";

    render(<LanguageSwitcher />);

    await user.click(screen.getByRole("button"));

    expect(await screen.findByRole("menuitem", { name: "English" })).toHaveClass(
      "font-bold",
    );
  });
});
