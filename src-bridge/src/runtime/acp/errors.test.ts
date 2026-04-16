import { describe, expect, test } from "bun:test";
import {
  AcpCapabilityUnsupported,
  AcpAuthMissing,
  AcpCommandNotFound,
  AcpProcessCrash,
} from "./errors.js";

describe("acp errors (extended)", () => {
  test("AcpCapabilityUnsupported carries method and reason", () => {
    const err = new AcpCapabilityUnsupported("setModel", "no_rpc_advertised");
    expect(err.name).toBe("AcpCapabilityUnsupported");
    expect(err.method).toBe("setModel");
    expect(err.reason).toBe("no_rpc_advertised");
  });

  test("AcpAuthMissing lists missing env vars", () => {
    const err = new AcpAuthMissing("claude_code", ["ANTHROPIC_API_KEY"]);
    expect(err.name).toBe("AcpAuthMissing");
    expect(err.adapterId).toBe("claude_code");
    expect(err.missingEnv).toEqual(["ANTHROPIC_API_KEY"]);
  });

  test("AcpCommandNotFound carries adapterId and command", () => {
    const err = new AcpCommandNotFound("gemini", "gemini");
    expect(err.name).toBe("AcpCommandNotFound");
    expect(err.adapterId).toBe("gemini");
    expect(err.command).toBe("gemini");
  });

  test("AcpProcessCrash carries stderr tail and exit info", () => {
    const err = new AcpProcessCrash("child exited", {
      stderrTail: "oom",
      exitCode: 137,
      signal: "SIGKILL",
    });
    expect(err.stderrTail).toBe("oom");
    expect(err.exitCode).toBe(137);
    expect(err.signal).toBe("SIGKILL");
  });
});
