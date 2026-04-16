import { describe, expect, test } from "bun:test";
import { mapSessionUpdate } from "./session-update.js";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const base = (update: any) => ({ sessionId: "s1", update } as any);

describe("mapSessionUpdate — stable variants", () => {
  test("agent_message_chunk → output with text", () => {
    const ev = mapSessionUpdate(
      base({
        sessionUpdate: "agent_message_chunk",
        content: { type: "text", text: "hi" },
      }),
    );
    expect(ev.type).toBe("output");
    expect(ev.text).toBe("hi");
    expect(ev.session_id).toBe("s1");
  });

  test("agent_thought_chunk → reasoning", () => {
    expect(
      mapSessionUpdate(
        base({
          sessionUpdate: "agent_thought_chunk",
          content: { type: "text", text: "think" },
        }),
      ).type,
    ).toBe("reasoning");
  });

  test("user_message_chunk → partial_message direction=user", () => {
    const ev = mapSessionUpdate(
      base({
        sessionUpdate: "user_message_chunk",
        content: { type: "text", text: "hi" },
      }),
    );
    expect(ev.type).toBe("partial_message");
    expect(ev.direction).toBe("user");
  });

  test("tool_call → tool_call (carries id + title)", () => {
    const ev = mapSessionUpdate(
      base({ sessionUpdate: "tool_call", toolCallId: "1", title: "Write" }),
    );
    expect(ev.type).toBe("tool_call");
    expect(ev.tool_call_id).toBe("1");
    expect(ev.title).toBe("Write");
  });

  test("tool_call_update completed → tool_result", () => {
    expect(
      mapSessionUpdate(
        base({
          sessionUpdate: "tool_call_update",
          status: "completed",
          toolCallId: "1",
        }),
      ).type,
    ).toBe("tool_result");
  });

  test("tool_call_update failed → tool_result", () => {
    expect(
      mapSessionUpdate(
        base({
          sessionUpdate: "tool_call_update",
          status: "failed",
          toolCallId: "1",
        }),
      ).type,
    ).toBe("tool_result");
  });

  test("tool_call_update in_progress → tool.status_change", () => {
    expect(
      mapSessionUpdate(
        base({
          sessionUpdate: "tool_call_update",
          status: "in_progress",
          toolCallId: "1",
        }),
      ).type,
    ).toBe("tool.status_change");
  });

  test("plan → todo_update", () => {
    const ev = mapSessionUpdate(
      base({ sessionUpdate: "plan", entries: [{ text: "step 1" }] }),
    );
    expect(ev.type).toBe("todo_update");
    expect(ev.entries).toEqual([{ text: "step 1" }]);
  });

  test("current_mode_update → status_change kind=mode", () => {
    const ev = mapSessionUpdate(
      base({ sessionUpdate: "current_mode_update", currentModeId: "ask" }),
    );
    expect(ev.type).toBe("status_change");
    expect(ev.kind).toBe("mode");
    expect(ev.mode_id).toBe("ask");
  });

  test("available_commands_update → status_change kind=commands", () => {
    const ev = mapSessionUpdate(
      base({
        sessionUpdate: "available_commands_update",
        availableCommands: ["build", "test"],
      }),
    );
    expect(ev.type).toBe("status_change");
    expect(ev.kind).toBe("commands");
    expect(ev.commands).toEqual(["build", "test"]);
  });

  test("config_option_update → status_change kind=config_option", () => {
    const ev = mapSessionUpdate(
      base({
        sessionUpdate: "config_option_update",
        configId: "model",
        value: "opus",
      }),
    );
    expect(ev.type).toBe("status_change");
    expect(ev.kind).toBe("config_option");
    expect(ev.option_id).toBe("model");
    expect(ev.value).toBe("opus");
  });

  test("session_info_update → status_change kind=session_info", () => {
    const ev = mapSessionUpdate(
      base({ sessionUpdate: "session_info_update", title: "My Session" }),
    );
    expect(ev.kind).toBe("session_info");
  });

  test("usage_update → cost_update", () => {
    const ev = mapSessionUpdate(
      base({
        sessionUpdate: "usage_update",
        usage: { inputTokens: 100, outputTokens: 50 },
      }),
    );
    expect(ev.type).toBe("cost_update");
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    expect((ev.usage as any).inputTokens).toBe(100);
  });
});

describe("mapSessionUpdate — fallback + metadata", () => {
  test("unknown sessionUpdate → acp_passthrough with _raw", () => {
    const raw = { sessionUpdate: "future_variant_xyz", payload: "abc" };
    const ev = mapSessionUpdate(base(raw));
    expect(ev.type).toBe("status_change");
    expect(ev.kind).toBe("acp_passthrough");
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    expect((ev.metadata as any)?._raw).toBe(raw);
  });

  test("_meta copied verbatim from notification", () => {
    const ev = mapSessionUpdate({
      sessionId: "s1",
      _meta: { adapter: "claude_code" },
      update: {
        sessionUpdate: "agent_message_chunk",
        content: { type: "text", text: "hi" },
      },
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    expect((ev.metadata as any)?._meta).toEqual({ adapter: "claude_code" });
  });

  test("_meta on update level captured when notification has none", () => {
    const ev = mapSessionUpdate(
      base({
        sessionUpdate: "agent_message_chunk",
        content: { type: "text", text: "hi" },
        _meta: { costTokens: 5 },
      }),
    );
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    expect((ev.metadata as any)?._meta).toEqual({ costTokens: 5 });
  });
});
