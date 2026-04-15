import { createStore } from "zustand/vanilla";
import {
  captureStoreSnapshot,
  isPlaywrightHarnessEnabled,
} from "./playwright-harness";

describe("playwright harness helpers", () => {
  describe("isPlaywrightHarnessEnabled", () => {
    it("disables the harness in production by default", () => {
      expect(
        isPlaywrightHarnessEnabled({
          nodeEnv: "production",
        }),
      ).toBe(false);
    });

    it("enables the harness during development", () => {
      expect(
        isPlaywrightHarnessEnabled({
          nodeEnv: "development",
        }),
      ).toBe(true);
    });

    it("allows an explicit override flag outside development", () => {
      expect(
        isPlaywrightHarnessEnabled({
          nodeEnv: "production",
          allowFlag: "1",
        }),
      ).toBe(true);
    });
  });

  it("restores a captured store snapshot after harness mutations", () => {
    const store = createStore<{
      count: number;
      label: string;
      increment: () => void;
    }>()((set) => ({
      count: 0,
      label: "initial",
      increment: () =>
        set((state) => ({
          count: state.count + 1,
        })),
    }));

    const restore = captureStoreSnapshot(store);

    store.setState(
      {
        count: 9,
        label: "playwright",
        increment: () => undefined,
      },
      true,
    );

    expect(store.getState().label).toBe("playwright");

    restore();

    expect(store.getState().label).toBe("initial");
    expect(store.getState().count).toBe(0);

    store.getState().increment();

    expect(store.getState().count).toBe(1);
  });
});
