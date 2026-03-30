import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeToggle } from "./theme-toggle";

const mockSetTheme = jest.fn();
let mockTheme = "system";

jest.mock("next-themes", () => ({
  useTheme: () => ({ theme: mockTheme, setTheme: mockSetTheme }),
}));

beforeEach(() => {
  jest.clearAllMocks();
  mockTheme = "system";
});

describe("ThemeToggle", () => {
  it("renders three theme options", () => {
    render(<ThemeToggle />);
    expect(screen.getByRole("button", { name: /light/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /dark/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /system/i })).toBeInTheDocument();
  });

  it("calls setTheme with 'dark' when dark button is clicked", async () => {
    render(<ThemeToggle />);
    await userEvent.click(screen.getByRole("button", { name: /dark/i }));
    expect(mockSetTheme).toHaveBeenCalledWith("dark");
  });

  it("calls setTheme with 'light' when light button is clicked", async () => {
    render(<ThemeToggle />);
    await userEvent.click(screen.getByRole("button", { name: /light/i }));
    expect(mockSetTheme).toHaveBeenCalledWith("light");
  });

  it("calls setTheme with 'system' when system button is clicked", async () => {
    mockTheme = "dark";
    render(<ThemeToggle />);
    await userEvent.click(screen.getByRole("button", { name: /system/i }));
    expect(mockSetTheme).toHaveBeenCalledWith("system");
  });
});
