import rolesMessages from "../../messages/en/roles.json";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string | number>) => {
    const resolved = key
      .split(".")
      .reduce<unknown>((current, part) => {
        if (current && typeof current === "object" && part in current) {
          return (current as Record<string, unknown>)[part];
        }
        return undefined;
      }, rolesMessages as unknown as Record<string, unknown>);

    if (typeof resolved !== "string") {
      return key;
    }
    return Object.entries(values ?? {}).reduce(
      (message, [name, value]) => message.replace(`{${name}}`, String(value)),
      resolved,
    );
  },
}));

import { render, screen } from "@testing-library/react";
import { ProvenanceBadge } from "./provenance-badge";

describe("ProvenanceBadge", () => {
  it("renders the provenance label with the mapped style", () => {
    render(<ProvenanceBadge provenance="template" className="extra" />);

    const badge = screen.getByText("Template");
    expect(badge).toHaveClass("bg-purple-500/15");
    expect(badge).toHaveClass("extra");
  });
});
