import { describe, expect, test } from "bun:test";
import {
  assertProviderCredentials,
  MissingProviderCredentialsError,
  resolveProviderSelection,
  UnsupportedProviderCapabilityError,
  UnknownProviderError,
} from "./registry.js";

describe("provider registry", () => {
  test("resolves default providers per capability", () => {
    const execute = resolveProviderSelection("agent_execution");
    const decompose = resolveProviderSelection("text_generation");

    expect(execute).toMatchObject({
      provider: "anthropic",
      model: "claude-sonnet-4-5",
      capability: "agent_execution",
    });
    expect(decompose).toMatchObject({
      provider: "anthropic",
      model: "claude-haiku-4-5",
      capability: "text_generation",
    });
  });

  test("rejects unknown providers and unsupported capabilities explicitly", () => {
    expect(() =>
      resolveProviderSelection("text_generation", { provider: "missing-provider" }),
    ).toThrow(UnknownProviderError);

    expect(() =>
      resolveProviderSelection("agent_execution", { provider: "openai" }),
    ).toThrow(UnsupportedProviderCapabilityError);
  });

  test("fails when required provider credentials are missing", () => {
    const selection = resolveProviderSelection("text_generation", {
      provider: "google",
    });

    expect(() => assertProviderCredentials(selection, {})).toThrow(
      MissingProviderCredentialsError,
    );
  });

  test("accepts Anthropic auth token as an alternative credential", () => {
    const selection = resolveProviderSelection("text_generation", {
      provider: "anthropic",
    });

    expect(() =>
      assertProviderCredentials(selection, {
        ANTHROPIC_AUTH_TOKEN: "auth-token",
      }),
    ).not.toThrow();
  });
});
