"use client";

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import LoginPage from "./page";
import { useAuthStore } from "@/lib/stores/auth-store";

const pushMock = jest.fn();
const replaceMock = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter() {
    return {
      push: pushMock,
      replace: replaceMock,
      prefetch: jest.fn(),
      back: jest.fn(),
      forward: jest.fn(),
    };
  },
}));

describe("LoginPage", () => {
  beforeEach(() => {
    pushMock.mockReset();
    replaceMock.mockReset();
    localStorage.clear();
    useAuthStore.setState({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "unauthenticated",
      hasHydrated: true,
      login: jest.fn().mockResolvedValue(undefined),
      bootstrapSession: jest.fn().mockResolvedValue(undefined),
    } as never);
  });

  it("normalizes the email before submitting login and navigates on success", async () => {
    const user = userEvent.setup();
    const loginMock = jest.fn().mockResolvedValue(undefined);
    useAuthStore.setState({ login: loginMock } as never);

    render(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), " Test@Example.COM ");
    await user.type(screen.getByLabelText("Password"), "password123");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    await waitFor(() => {
      expect(loginMock).toHaveBeenCalledWith("test@example.com", "password123");
    });
    expect(pushMock).toHaveBeenCalledWith("/");
  });

  it("redirects authenticated users away from the login page", async () => {
    useAuthStore.setState({
      accessToken: "access-1",
      refreshToken: "refresh-1",
      user: {
        id: "user-1",
        email: "test@example.com",
        name: "Test User",
      },
      status: "authenticated",
      hasHydrated: true,
    } as never);

    render(<LoginPage />);

    await waitFor(() => {
      expect(replaceMock).toHaveBeenCalledWith("/");
    });
  });

  it("shows the backend error when login fails", async () => {
    const user = userEvent.setup();
    const loginMock = jest.fn().mockRejectedValue(new Error("invalid email or password"));
    useAuthStore.setState({ login: loginMock } as never);

    render(<LoginPage />);

    await user.type(screen.getByLabelText("Email"), "test@example.com");
    await user.type(screen.getByLabelText("Password"), "wrong-password");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(await screen.findByText("invalid email or password")).toBeInTheDocument();
    expect(pushMock).not.toHaveBeenCalled();
  });

  it("bootstraps the current session before showing the form when auth is unresolved", async () => {
    const bootstrapSessionMock = jest.fn().mockResolvedValue(undefined);
    useAuthStore.setState({
      status: "idle",
      hasHydrated: true,
      bootstrapSession: bootstrapSessionMock,
    } as never);

    render(<LoginPage />);

    await waitFor(() => {
      expect(bootstrapSessionMock).toHaveBeenCalledTimes(1);
    });
    expect(screen.getByText("Checking your session...")).toBeInTheDocument();
  });
});
