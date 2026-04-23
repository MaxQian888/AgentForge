import { fireEvent, render, screen, act } from "@testing-library/react";
import { MarketplaceSearchBar } from "./marketplace-search-bar";

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string) => {
    const map: Record<string, string> = {
      "search.placeholder": "Search marketplace...",
    };
    return map[key] ?? key;
  },
}));

describe("MarketplaceSearchBar", () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it("renders an input with placeholder text", () => {
    render(
      <MarketplaceSearchBar value="" onChange={jest.fn()} onSearch={jest.fn()} />,
    );
    expect(screen.getByPlaceholderText("Search marketplace...")).toBeInTheDocument();
  });

  it("calls onChange on input change", () => {
    const onChange = jest.fn();
    render(
      <MarketplaceSearchBar value="" onChange={onChange} onSearch={jest.fn()} />,
    );
    fireEvent.change(screen.getByPlaceholderText("Search marketplace..."), {
      target: { value: "test" },
    });
    expect(onChange).toHaveBeenCalledWith("test");
  });

  it("debounces onSearch by 300ms on input change", () => {
    const onSearch = jest.fn();
    render(
      <MarketplaceSearchBar value="" onChange={jest.fn()} onSearch={onSearch} />,
    );
    fireEvent.change(screen.getByPlaceholderText("Search marketplace..."), {
      target: { value: "query" },
    });
    expect(onSearch).not.toHaveBeenCalled();

    act(() => {
      jest.advanceTimersByTime(300);
    });

    expect(onSearch).toHaveBeenCalledWith("query");
  });

  it("fires onSearch immediately on Enter key", () => {
    const onSearch = jest.fn();
    render(
      <MarketplaceSearchBar value="instant" onChange={jest.fn()} onSearch={onSearch} />,
    );
    fireEvent.keyDown(screen.getByPlaceholderText("Search marketplace..."), {
      key: "Enter",
    });
    expect(onSearch).toHaveBeenCalledWith("instant");
  });
});
