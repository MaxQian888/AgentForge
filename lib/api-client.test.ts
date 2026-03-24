import { createApiClient, ApiError } from "./api-client";

const BASE = "http://localhost:7777";

function mockFetch(status: number, body: unknown): typeof fetch {
  return jest.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  }) as unknown as typeof fetch;
}

beforeEach(() => {
  jest.restoreAllMocks();
});

describe("createApiClient", () => {
  const api = createApiClient(BASE);

  describe("get", () => {
    it("sends GET and returns data", async () => {
      global.fetch = mockFetch(200, { id: 1 });

      const res = await api.get("/users");

      expect(fetch).toHaveBeenCalledWith(
        `${BASE}/users`,
        expect.objectContaining({ method: "GET" })
      );
      expect(res).toEqual({ data: { id: 1 }, status: 200 });
    });

    it("sends Authorization header when token provided", async () => {
      global.fetch = mockFetch(200, {});

      await api.get("/me", { token: "tok123" });

      expect(fetch).toHaveBeenCalledWith(
        `${BASE}/me`,
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer tok123",
          }),
        })
      );
    });
  });

  describe("post", () => {
    it("sends POST with JSON body", async () => {
      global.fetch = mockFetch(201, { id: 1 });

      const res = await api.post("/users", { name: "Alice" });

      expect(fetch).toHaveBeenCalledWith(
        `${BASE}/users`,
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ name: "Alice" }),
        })
      );
      expect(res.status).toBe(201);
    });

    it("sends Authorization header when token provided", async () => {
      global.fetch = mockFetch(201, {});

      await api.post("/users", {}, { token: "tok" });

      expect(fetch).toHaveBeenCalledWith(
        `${BASE}/users`,
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer tok",
          }),
        })
      );
    });
  });

  describe("put", () => {
    it("sends PUT with JSON body", async () => {
      global.fetch = mockFetch(200, { updated: true });

      const res = await api.put("/users/1", { name: "Bob" });

      expect(fetch).toHaveBeenCalledWith(
        `${BASE}/users/1`,
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ name: "Bob" }),
        })
      );
      expect(res.data).toEqual({ updated: true });
    });

    it("sends Authorization header when token provided", async () => {
      global.fetch = mockFetch(200, {});

      await api.put("/users/1", {}, { token: "t" });

      expect(fetch).toHaveBeenCalledWith(
        `${BASE}/users/1`,
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer t",
          }),
        })
      );
    });
  });

  describe("delete", () => {
    it("sends DELETE request", async () => {
      global.fetch = mockFetch(200, { deleted: true });

      const res = await api.delete("/users/1");

      expect(fetch).toHaveBeenCalledWith(
        `${BASE}/users/1`,
        expect.objectContaining({ method: "DELETE" })
      );
      expect(res.data).toEqual({ deleted: true });
    });

    it("sends Authorization header when token provided", async () => {
      global.fetch = mockFetch(200, {});

      await api.delete("/users/1", { token: "t" });

      expect(fetch).toHaveBeenCalledWith(
        `${BASE}/users/1`,
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer t",
          }),
        })
      );
    });
  });

  describe("error handling", () => {
    it("throws ApiError on non-ok response with server message", async () => {
      global.fetch = mockFetch(401, { message: "unauthorized" });

      await expect(api.get("/secret")).rejects.toThrow(ApiError);
      await expect(api.get("/secret")).rejects.toThrow("unauthorized");
    });

    it("throws ApiError with HTTP status when no message in body", async () => {
      global.fetch = mockFetch(500, {});

      try {
        await api.get("/fail");
        fail("should have thrown");
      } catch (e) {
        expect(e).toBeInstanceOf(ApiError);
        expect((e as ApiError).status).toBe(500);
        expect((e as ApiError).message).toBe("HTTP 500");
      }
    });

    it("handles response with unparseable JSON", async () => {
      global.fetch = jest.fn().mockResolvedValue({
        ok: false,
        status: 502,
        json: () => Promise.reject(new Error("bad json")),
      }) as unknown as typeof fetch;

      try {
        await api.get("/bad");
        fail("should have thrown");
      } catch (e) {
        expect(e).toBeInstanceOf(ApiError);
        expect((e as ApiError).status).toBe(502);
      }
    });
  });

  describe("wsUrl", () => {
    it("converts http to ws", () => {
      expect(api.wsUrl("/ws")).toBe("ws://localhost:7777/ws");
    });

    it("converts https to wss", () => {
      const secureApi = createApiClient("https://example.com");
      expect(secureApi.wsUrl("/ws")).toBe("wss://example.com/ws");
    });

    it("appends token as query param", () => {
      expect(api.wsUrl("/ws", "mytoken")).toBe(
        "ws://localhost:7777/ws?token=mytoken"
      );
    });
  });

  describe("base URL handling", () => {
    it("strips trailing slash from base URL", async () => {
      const api2 = createApiClient("http://localhost:7777/");
      global.fetch = mockFetch(200, {});

      await api2.get("/test");

      expect(fetch).toHaveBeenCalledWith(
        "http://localhost:7777/test",
        expect.anything()
      );
    });
  });
});
