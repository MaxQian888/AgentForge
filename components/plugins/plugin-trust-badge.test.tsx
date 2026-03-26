import { render, screen } from "@testing-library/react";
import { PluginTrustBadge } from "./plugin-trust-badge";

describe("PluginTrustBadge", () => {
  it("renders trust, approval, and update indicators", () => {
    const { container } = render(
      <PluginTrustBadge
        source={{
          type: "catalog",
          digest: "sha256:test",
          signature: "sigstore-bundle",
          trust: {
            status: "verified",
            approvalState: "approved",
          },
          release: {
            availableVersion: "1.1.0",
          },
        }}
      />,
    );

    expect(screen.getByText("verified")).toBeInTheDocument();
    expect(screen.getByText("approved")).toBeInTheDocument();
    expect(screen.getByText("Update available: v1.1.0")).toBeInTheDocument();
    expect(container.querySelectorAll("svg")).toHaveLength(3);
  });

  it("falls back to unknown trust and not-required approval", () => {
    render(<PluginTrustBadge source={{ type: "local" }} />);

    expect(screen.getByText("unknown")).toBeInTheDocument();
    expect(screen.getByText("not-required")).toBeInTheDocument();
    expect(screen.queryByText(/Update available/i)).not.toBeInTheDocument();
  });
});
