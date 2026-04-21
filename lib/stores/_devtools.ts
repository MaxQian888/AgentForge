import { devtools, type DevtoolsOptions } from "zustand/middleware";
import type { StateCreator, StoreMutatorIdentifier } from "zustand";

/**
 * Conditionally wraps a Zustand state creator with the `devtools` middleware.
 * Active in development builds; a pass-through in production.
 *
 * Usage with zustand v5 requires the currying form:
 *   create<State>()(withDevtools((set) => ({ ... }), { name: "my-store" }))
 *
 * When composing with other middleware (e.g. persist), pass the inner
 * middleware as the initializer — withDevtools is the outermost wrapper:
 *   create<State>()(withDevtools(persist(...), { name: "my-store" }))
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AnyStateCreator = StateCreator<any, any, any>;

export function withDevtools<
  T,
  Mps extends [StoreMutatorIdentifier, unknown][] = [],
  Mcs extends [StoreMutatorIdentifier, unknown][] = [],
>(
  initializer: StateCreator<T, Mps, Mcs>,
  options?: DevtoolsOptions,
): StateCreator<T, Mps, [["zustand/devtools", never], ...Mcs]> {
  if (process.env.NODE_ENV === "production") {
    return initializer as unknown as StateCreator<
      T,
      Mps,
      [["zustand/devtools", never], ...Mcs]
    >;
  }
  return devtools(initializer as AnyStateCreator, options) as unknown as StateCreator<
    T,
    Mps,
    [["zustand/devtools", never], ...Mcs]
  >;
}
