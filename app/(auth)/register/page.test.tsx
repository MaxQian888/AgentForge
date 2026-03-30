"use client";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import RegisterPage from "./page";
import { useAuthStore } from "@/lib/stores/auth-store";

const pushMock = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter() {
    return {
      push: pushMock,
      replace: jest.fn(),
      prefetch: jest.fn(),
      back: jest.fn(),
      forward: jest.fn(),
    };
  },
}));

describe("RegisterPage", () => {
  beforeEach(() => {
    pushMock.mockReset();
    localStorage.clear();
    useAuthStore.setState({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "unauthenticated",
      hasHydrated: true,
      register: jest.fn().mockResolvedValue(undefined),
      bootstrapSession: jest.fn().mockResolvedValue(undefined),
    } as never);
  });

  it("submits the registration form and navigates to the dashboard on success", async () => {
    const user = userEvent.setup();
    const registerMock = jest.fn().mockResolvedValue(undefined);
    useAuthStore.setState({ register: registerMock } as never);

    render(<RegisterPage />);

    await user.type(screen.getByLabelText(/name/i), "Ada Lovelace");
    await user.type(screen.getByLabelText("Email"), "ada@example.com");
    await user.type(screen.getByLabelText("Password"), "strong-password");
    await user.click(screen.getByRole("button"));

    await waitFor(() => {
      expect(registerMock).toHaveBeenCalledWith(
        "ada@example.com",
        "strong-password",
        "Ada Lovelace",
      );
    });
    expect(pushMock).toHaveBeenCalledWith("/");
  });

  it("shows the backend error when registration fails", async () => {
    const user = userEvent.setup();
    const registerMock = jest.fn().mockRejectedValue(new Error("email already exists"));
    useAuthStore.setState({ register: registerMock } as never);

    render(<RegisterPage />);

    await user.type(screen.getByLabelText(/name/i), "Ada Lovelace");
    await user.type(screen.getByLabelText("Email"), "ada@example.com");
    await user.type(screen.getByLabelText("Password"), "strong-password");
    await user.click(screen.getByRole("button"));

    expect(await screen.findByText("email already exists")).toBeInTheDocument();
    expect(pushMock).not.toHaveBeenCalled();
  });
});
