import type { StoreApi } from "zustand";

type PlaywrightHarnessEnv = {
  nodeEnv?: string;
  allowFlag?: string;
};

export function isPlaywrightHarnessEnabled({
  nodeEnv = process.env.NODE_ENV,
  allowFlag = process.env.ENABLE_PLAYWRIGHT_HARNESS,
}: PlaywrightHarnessEnv = {}): boolean {
  return (
    nodeEnv === "development" ||
    nodeEnv === "test" ||
    allowFlag === "1" ||
    allowFlag === "true"
  );
}

export function captureStoreSnapshot<TState>(
  store: Pick<StoreApi<TState>, "getState" | "setState">,
): () => void {
  const snapshot = store.getState();
  return () => {
    store.setState(snapshot, true);
  };
}
