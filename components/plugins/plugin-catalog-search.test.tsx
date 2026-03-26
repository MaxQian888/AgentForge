import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PluginCatalogSearch } from "./plugin-catalog-search";
import type { MarketplacePluginEntry } from "@/lib/stores/plugin-store";

const searchCatalog = jest.fn();
const setCatalogQuery = jest.fn();

const storeState: {
  searchCatalog: typeof searchCatalog;
  catalogResults: MarketplacePluginEntry[];
  catalogQuery: string;
  setCatalogQuery: typeof setCatalogQuery;
} = {
  searchCatalog,
  catalogResults: [],
  catalogQuery: "",
  setCatalogQuery,
};

jest.mock("@/lib/stores/plugin-store", () => ({
  usePluginStore: (
    selector: (state: typeof storeState) => unknown,
  ) => selector(storeState),
}));

describe("PluginCatalogSearch", () => {
  beforeEach(() => {
    jest.useRealTimers();
    searchCatalog.mockReset().mockResolvedValue(undefined);
    setCatalogQuery.mockReset();
    storeState.catalogResults = [];
    storeState.catalogQuery = "";
  });

  it("shows the empty query prompt and clears the stored query", () => {
    render(<PluginCatalogSearch onSelect={jest.fn()} />);

    expect(screen.getByText("Search the plugin catalog")).toBeInTheDocument();
    expect(setCatalogQuery).toHaveBeenCalledWith("");
  });

  it("debounces catalog lookups and trims the search text", async () => {
    jest.useFakeTimers();

    render(<PluginCatalogSearch onSelect={jest.fn()} />);

    fireEvent.change(screen.getByPlaceholderText("Search the plugin catalog..."), {
      target: { value: "  repo search  " },
    });

    act(() => {
      jest.advanceTimersByTime(300);
    });

    await waitFor(() => {
      expect(searchCatalog).toHaveBeenCalledWith("repo search");
    });
  });

  it("renders catalog results and forwards the selected entry", async () => {
    const user = userEvent.setup();
    const onSelect = jest.fn();

    storeState.catalogQuery = "repo";
    storeState.catalogResults = [
      {
        id: "repo-search",
        name: "Repo Search",
        description: "Searches the workspace",
        version: "1.0.0",
        author: "AgentForge",
        kind: "tool",
      },
    ];

    render(<PluginCatalogSearch onSelect={onSelect} />);

    await user.click(screen.getByRole("button", { name: /Repo Search/i }));

    expect(onSelect).toHaveBeenCalledWith(storeState.catalogResults[0]);
  });
});
