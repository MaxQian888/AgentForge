import { act, fireEvent, renderHook } from "@testing-library/react";

const pushMock = jest.fn();

jest.mock("next/navigation", () => ({
  useRouter: () => ({
    push: pushMock,
  }),
}));

import { useKeyboardNavigation } from "./use-keyboard-navigation";

describe("useKeyboardNavigation", () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.useFakeTimers();
    document.body.innerHTML = "";
  });

  afterEach(() => {
    jest.runOnlyPendingTimers();
    jest.useRealTimers();
  });

  it("navigates to the mapped route after the go-to shortcut sequence", () => {
    renderHook(() => useKeyboardNavigation());

    fireEvent.keyDown(document, { key: "g" });

    const followUpEvent = new KeyboardEvent("keydown", {
      key: "p",
      bubbles: true,
      cancelable: true,
    });

    document.dispatchEvent(followUpEvent);

    expect(pushMock).toHaveBeenCalledWith("/projects");
    expect(followUpEvent.defaultPrevented).toBe(true);
  });

  it("ignores shortcut keys from editable targets and modified key presses", () => {
    renderHook(() => useKeyboardNavigation());

    const input = document.createElement("input");
    document.body.appendChild(input);

    fireEvent.keyDown(input, { key: "g" });
    fireEvent.keyDown(document, { key: "g", ctrlKey: true });
    fireEvent.keyDown(document, { key: "p" });

    expect(pushMock).not.toHaveBeenCalled();
  });

  it("expires pending shortcuts and unregisters the keydown listener on unmount", () => {
    const { unmount } = renderHook(() => useKeyboardNavigation());

    fireEvent.keyDown(document, { key: "g" });

    act(() => {
      jest.advanceTimersByTime(1001);
    });

    fireEvent.keyDown(document, { key: "d" });

    expect(pushMock).not.toHaveBeenCalled();

    unmount();

    fireEvent.keyDown(document, { key: "g" });
    fireEvent.keyDown(document, { key: "d" });

    expect(pushMock).not.toHaveBeenCalled();
  });
});
