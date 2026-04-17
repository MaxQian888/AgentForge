import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SectionAccessibility } from "./section-accessibility";
import {
  DEFAULT_APPEARANCE,
  useAppearanceStore,
} from "@/lib/stores/appearance-store";

describe("SectionAccessibility", () => {
  const originalMatchMedia = window.matchMedia;

  beforeEach(() => {
    localStorage.clear();
    useAppearanceStore.setState({ ...DEFAULT_APPEARANCE });
    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      writable: true,
      value: jest.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: jest.fn(),
        removeListener: jest.fn(),
        addEventListener: jest.fn(),
        removeEventListener: jest.fn(),
        dispatchEvent: jest.fn(),
      })),
    });
  });

  afterEach(() => {
    Object.defineProperty(window, "matchMedia", {
      configurable: true,
      writable: true,
      value: originalMatchMedia,
    });
  });

  it("renders motion, contrast and screen reader sections", () => {
    render(<SectionAccessibility />);
    expect(screen.getByRole("heading", { name: "Accessibility" })).toBeInTheDocument();
    expect(screen.getByText("Reduced Motion")).toBeInTheDocument();
    expect(screen.getAllByText("High Contrast").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Screen Reader Mode").length).toBeGreaterThan(0);
  });

  it("toggles reduced motion via radio group", async () => {
    const user = userEvent.setup();
    render(<SectionAccessibility />);
    await user.click(screen.getByRole("radio", { name: "Reduce" }));
    expect(useAppearanceStore.getState().motionPreference).toBe("reduce");
  });

  it("falls back to system motion preference", async () => {
    const user = userEvent.setup();
    useAppearanceStore.setState({ motionPreference: "reduce" });
    render(<SectionAccessibility />);
    await user.click(screen.getByRole("radio", { name: "System" }));
    expect(useAppearanceStore.getState().motionPreference).toBe("system");
  });

  it("toggles high contrast switch", async () => {
    const user = userEvent.setup();
    render(<SectionAccessibility />);
    const toggle = screen.getByRole("switch", { name: "High Contrast" });
    expect(toggle).toHaveAttribute("data-state", "unchecked");
    await user.click(toggle);
    expect(useAppearanceStore.getState().highContrast).toBe(true);
  });

  it("toggles screen reader switch", async () => {
    const user = userEvent.setup();
    render(<SectionAccessibility />);
    const toggle = screen.getByRole("switch", { name: "Screen Reader Mode" });
    await user.click(toggle);
    expect(useAppearanceStore.getState().screenReaderMode).toBe(true);
  });

  it("resets appearance via the reset button", async () => {
    const user = userEvent.setup();
    useAppearanceStore.setState({
      motionPreference: "reduce",
      highContrast: true,
      screenReaderMode: true,
      density: "compact",
    });
    render(<SectionAccessibility />);
    await user.click(screen.getByRole("button", { name: "Reset appearance" }));
    const state = useAppearanceStore.getState();
    expect(state.motionPreference).toBe("system");
    expect(state.highContrast).toBe(false);
    expect(state.screenReaderMode).toBe(false);
    expect(state.density).toBe("comfortable");
  });

  it("reflects system reduced motion preference in status badge", () => {
    (window.matchMedia as jest.Mock).mockImplementation((query: string) => ({
      matches: query === "(prefers-reduced-motion: reduce)",
      media: query,
      onchange: null,
      addListener: jest.fn(),
      removeListener: jest.fn(),
      addEventListener: jest.fn(),
      removeEventListener: jest.fn(),
      dispatchEvent: jest.fn(),
    }));
    render(<SectionAccessibility />);
    const badge = screen.getByTestId("motion-system-status");
    expect(badge).toHaveTextContent("System prefers reduced motion");
  });
});
