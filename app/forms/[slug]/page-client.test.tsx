"use client";

import { render, screen, waitFor } from "@testing-library/react";
import { PublicFormPageClient } from "./page-client";
import { createApiClient } from "@/lib/api-client";
import { useAuthStore } from "@/lib/stores/auth-store";
import { useFormStore } from "@/lib/stores/form-store";

const replaceMock = jest.fn();

jest.mock("@/lib/api-client", () => ({
  createApiClient: jest.fn(),
}));

jest.mock("next/navigation", () => ({
  useParams() {
    return { slug: "bug-report" };
  },
  useRouter() {
    return {
      replace: replaceMock,
      push: jest.fn(),
      prefetch: jest.fn(),
      back: jest.fn(),
      forward: jest.fn(),
    };
  },
}));

describe("PublicFormPageClient", () => {
  beforeEach(() => {
    replaceMock.mockReset();
    localStorage.clear();
    useFormStore.setState({
      formsByProject: {},
      formsBySlug: {},
      loadingByProject: {},
    } as never);
    useAuthStore.setState({
      accessToken: null,
      refreshToken: null,
      user: null,
      status: "unauthenticated",
      hasHydrated: true,
      bootstrapSession: jest.fn().mockResolvedValue(undefined),
    } as never);
  });

  it("renders a public form fetched by slug without requiring dashboard context", async () => {
    const api = { get: jest.fn(), post: jest.fn(), put: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({
      data: {
        id: "form-1",
        projectId: "project-1",
        name: "Bug Report",
        slug: "bug-report",
        fields: [{ key: "title", label: "Title" }],
        targetStatus: "inbox",
        isPublic: true,
        createdAt: "",
        updatedAt: "",
      },
    });

    render(<PublicFormPageClient slug="bug-report" />);

    expect(await screen.findByText("Bug Report")).toBeInTheDocument();
    expect(screen.getByText("Submit this form to create a task in this project.")).toBeInTheDocument();
    expect(api.get).toHaveBeenCalledWith("/api/v1/forms/bug-report", undefined);
  });

  it("redirects to login when the form fetch is unauthorized", async () => {
    const api = { get: jest.fn(), post: jest.fn(), put: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockRejectedValue(Object.assign(new Error("authentication required"), { status: 401 }));

    render(<PublicFormPageClient slug="bug-report" />);

    await waitFor(() => {
      expect(replaceMock).toHaveBeenCalledWith("/login");
    });
  });

  it("bootstraps the current session before fetching a form when auth is unresolved", async () => {
    const api = { get: jest.fn(), post: jest.fn(), put: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockResolvedValue({
      data: {
        id: "form-1",
        projectId: "project-1",
        name: "Bug Report",
        slug: "bug-report",
        fields: [],
        targetStatus: "inbox",
        isPublic: true,
        createdAt: "",
        updatedAt: "",
      },
    });
    const bootstrapSessionMock = jest.fn().mockImplementation(async () => {
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
    });
    useAuthStore.setState({
      accessToken: null,
      refreshToken: "refresh-1",
      user: null,
      status: "idle",
      hasHydrated: true,
      bootstrapSession: bootstrapSessionMock,
    } as never);

    render(<PublicFormPageClient slug="bug-report" />);

    await waitFor(() => {
      expect(bootstrapSessionMock).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith("/api/v1/forms/bug-report", { token: "access-1" });
    });
  });

  it("shows a retryable unavailable state when the form fetch fails for a non-auth reason", async () => {
    const api = { get: jest.fn(), post: jest.fn(), put: jest.fn(), delete: jest.fn() };
    (createApiClient as jest.Mock).mockReturnValue(api);
    api.get.mockRejectedValue(new Error("Form service unavailable"));
    useFormStore.setState({
      formsBySlug: {
        "bug-report": {
          id: "form-1",
          projectId: "project-1",
          name: "Bug Report",
          slug: "bug-report",
          fields: [],
          targetStatus: "inbox",
          isPublic: true,
          createdAt: "",
          updatedAt: "",
        },
      },
    } as never);

    render(<PublicFormPageClient slug="bug-report" />);

    expect(await screen.findByText("Form unavailable")).toBeInTheDocument();
    expect(screen.getByText("Form service unavailable")).toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });
});
