import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SectionAppearance } from "./section-appearance";
import {
  DEFAULT_APPEARANCE,
  useAppearanceStore,
} from "@/lib/stores/appearance-store";

jest.mock("@/lib/theme/provider", () => ({
  useTheme: () => ({ theme: "system", setTheme: jest.fn() }),
}));

const setLocale = jest.fn();
jest.mock("@/lib/stores/locale-store", () => ({
  useLocaleStore: (selector: (s: { locale: string; setLocale: typeof setLocale }) => unknown) =>
    selector({ locale: "en", setLocale }),
  SUPPORTED_LOCALES: ["en", "zh-CN"],
  DEFAULT_LOCALE: "en",
}));

describe("SectionAppearance", () => {
  beforeEach(() => {
    localStorage.clear();
    useAppearanceStore.setState({ ...DEFAULT_APPEARANCE });
  });

  it("renders heading and preview card", () => {
    render(<SectionAppearance />);
    expect(screen.getByRole("heading", { name: "Appearance" })).toBeInTheDocument();
    expect(screen.getByTestId("appearance-preview")).toBeInTheDocument();
  });

  it("renders three density radio options", () => {
    render(<SectionAppearance />);
    expect(screen.getByRole("radio", { name: "Compact" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Comfortable" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Spacious" })).toBeInTheDocument();
  });

  it("selecting compact updates the store and preview data attribute", async () => {
    const user = userEvent.setup();
    render(<SectionAppearance />);
    await user.click(screen.getByRole("radio", { name: "Compact" }));
    expect(useAppearanceStore.getState().density).toBe("compact");
    const preview = screen.getByTestId("appearance-preview");
    expect(
      preview.querySelector("[data-density-preview]")?.getAttribute("data-density-preview"),
    ).toBe("compact");
  });

  it("selecting spacious updates the store", async () => {
    const user = userEvent.setup();
    render(<SectionAppearance />);
    await user.click(screen.getByRole("radio", { name: "Spacious" }));
    expect(useAppearanceStore.getState().density).toBe("spacious");
  });

  it("hovering a density option previews it without committing", () => {
    render(<SectionAppearance />);
    const spaciousRow = document.querySelector(
      '[data-density-option="spacious"]',
    ) as HTMLElement | null;
    expect(spaciousRow).not.toBeNull();

    expect(useAppearanceStore.getState().density).toBe("comfortable");

    act(() => {
      fireEvent.mouseEnter(spaciousRow!);
    });

    const preview = screen.getByTestId("appearance-preview");
    const card = preview.querySelector("[data-density-preview]");
    expect(card?.getAttribute("data-density-preview")).toBe("spacious");
    expect(useAppearanceStore.getState().density).toBe("comfortable");

    act(() => {
      fireEvent.mouseLeave(spaciousRow!);
    });
    expect(card?.getAttribute("data-density-preview")).toBe("comfortable");
  });
});
