import enMessages from "@/messages/en";
import zhCNMessages from "@/messages/zh-CN";

describe("message bundle normalization", () => {
  it("converts dotted namespace keys into nested objects for next-intl", () => {
    expect(enMessages).toMatchObject({
      dashboard: {
        error: {
          title: expect.any(String),
        },
      },
      agents: {
        monitor: {
          title: expect.any(String),
        },
      },
      tasks: {
        detail: {
          startAgent: expect.any(String),
        },
      },
    });

    expect(zhCNMessages).toMatchObject({
      dashboard: {
        error: {
          title: expect.any(String),
        },
      },
      agents: {
        monitor: {
          title: expect.any(String),
        },
      },
      tasks: {
        detail: {
          startAgent: expect.any(String),
        },
      },
    });

    expect(enMessages).toMatchObject({
      im: {
        eventLabels: {
          task: {
            created: expect.any(String),
          },
        },
      },
    });

    expect(zhCNMessages).toMatchObject({
      im: {
        eventLabels: {
          task: {
            created: expect.any(String),
          },
        },
      },
    });
  });
});
