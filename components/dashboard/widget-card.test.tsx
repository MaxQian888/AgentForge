import { WidgetCard } from "./widget-card";
import { WidgetWrapper } from "./widget-wrapper";

describe("WidgetCard", () => {
  it("re-exports WidgetWrapper as the canonical widget card component", () => {
    expect(WidgetCard).toBe(WidgetWrapper);
  });
});
