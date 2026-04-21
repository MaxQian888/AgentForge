/**
 * @jest-environment jsdom
 */
import { createBrowserLogger } from "./log";

describe("browser logger", () => {
  it("batches and POSTs warn+ to /api/v1/internal/logs/ingest", async () => {
    const calls: Array<{ url: string; init: RequestInit }> = [];
    const fetchMock: typeof fetch = (async (input, init) => {
      calls.push({ url: typeof input === "string" ? input : input.toString(), init: init! });
      return new Response(null, { status: 202 });
    }) as typeof fetch;

    const log = createBrowserLogger({
      fetch: fetchMock,
      flushMs: 5,
      bufferSize: 10,
      traceId: () => "tr_front000000000000000000",
    });
    log.info("ignored", { k: 1 });
    log.warn("kept", { k: 2 });
    log.error("kept2", { k: 3 });

    await new Promise((r) => setTimeout(r, 30));

    expect(calls).toHaveLength(1);
    expect(calls[0]!.url).toContain("/api/v1/internal/logs/ingest");
    const headers = new Headers(calls[0]!.init.headers);
    expect(headers.get("X-Trace-ID")).toBe("tr_front000000000000000000");
    const body = JSON.parse(String(calls[0]!.init.body));
    expect(Array.isArray(body)).toBe(true);
    expect(body).toHaveLength(2);
    expect(body[0].level).toBe("warn");
    expect(body[1].level).toBe("error");
    expect(body[0].summary).toBe("kept");
    expect(body[0].detail).toEqual({ k: 2, trace_id: "tr_front000000000000000000" });
  });

  it("flushes immediately when buffer fills", async () => {
    const calls: Array<{ init: RequestInit }> = [];
    const fetchMock: typeof fetch = (async (_i, init) => {
      calls.push({ init: init! });
      return new Response(null, { status: 202 });
    }) as typeof fetch;

    const log = createBrowserLogger({
      fetch: fetchMock,
      flushMs: 10000, // long
      bufferSize: 2,
      traceId: () => "",
    });
    log.warn("a");
    log.warn("b"); // hits bufferSize → immediate flush
    await new Promise((r) => setTimeout(r, 5));
    expect(calls).toHaveLength(1);
    const body = JSON.parse(String(calls[0]!.init.body));
    expect(body).toHaveLength(2);
  });

  it("never throws on fetch failure", async () => {
    const fetchMock: typeof fetch = (async () => {
      throw new Error("offline");
    }) as typeof fetch;
    const log = createBrowserLogger({
      fetch: fetchMock,
      flushMs: 5,
      bufferSize: 10,
      traceId: () => "",
    });
    log.error("boom");
    await new Promise((r) => setTimeout(r, 20));
    // getting here without a thrown exception is success
  });

  it("stamps per-entry trace_id at push time, not flush time", async () => {
    const calls: Array<{ init: RequestInit }> = [];
    const fetchMock: typeof fetch = (async (_i, init) => {
      calls.push({ init: init! });
      return new Response(null, { status: 202 });
    }) as typeof fetch;

    let currentTrace = "tr_first0000000000000000000";
    const log = createBrowserLogger({
      fetch: fetchMock,
      flushMs: 20,
      bufferSize: 10,
      traceId: () => currentTrace,
    });

    log.warn("a");                                  // stamped with tr_first
    currentTrace = "tr_second00000000000000000";
    log.error("b");                                 // stamped with tr_second

    await new Promise((r) => setTimeout(r, 40));
    expect(calls).toHaveLength(1);
    const body = JSON.parse(String(calls[0]!.init.body));
    expect(body).toHaveLength(2);
    expect(body[0].detail?.trace_id).toBe("tr_first0000000000000000000");
    expect(body[1].detail?.trace_id).toBe("tr_second00000000000000000");
    expect(body[0].summary).toBe("a");
    expect(body[1].summary).toBe("b");
  });
});
