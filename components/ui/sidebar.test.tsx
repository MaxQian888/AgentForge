import * as React from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import {
  Sidebar,
  SidebarContent,
  SidebarProvider,
} from "./sidebar";

const isMobileMock = jest.fn();

jest.mock("@/hooks/use-mobile", () => ({
  useIsMobile: () => isMobileMock(),
}));

jest.mock("./sheet", () => {
  const SheetContext = React.createContext(false);

  return {
    Sheet: ({
      open,
      children,
    }: {
      open?: boolean;
      children: React.ReactNode;
    }) => (
      <SheetContext.Provider value={Boolean(open)}>
        {children}
      </SheetContext.Provider>
    ),
    SheetContent: ({
      children,
      ...props
    }: React.HTMLAttributes<HTMLDivElement>) => {
      const open = React.useContext(SheetContext);

      if (!open) {
        return null;
      }

      return (
        <div data-testid="mobile-sidebar-sheet" {...props}>
          {children}
        </div>
      );
    },
    SheetHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
    SheetTitle: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
    SheetDescription: ({ children }: { children: React.ReactNode }) => (
      <div>{children}</div>
    ),
  };
});

function renderMobileSidebar() {
  return render(
    <SidebarProvider>
      <Sidebar>
        <SidebarContent>
          <div>Navigation content</div>
        </SidebarContent>
      </Sidebar>
    </SidebarProvider>,
  );
}

describe("Sidebar mobile swipe gestures", () => {
  beforeEach(() => {
    isMobileMock.mockReturnValue(true);
  });

  it("opens the mobile drawer when the user swipes from the left edge", () => {
    renderMobileSidebar();

    expect(screen.queryByTestId("mobile-sidebar-sheet")).not.toBeInTheDocument();

    const edge = screen.getByTestId("sidebar-swipe-edge");
    fireEvent.touchStart(edge, {
      touches: [{ clientX: 8, clientY: 120 }],
    });
    fireEvent.touchMove(edge, {
      touches: [{ clientX: 96, clientY: 126 }],
    });
    fireEvent.touchEnd(edge, {
      changedTouches: [{ clientX: 96, clientY: 126 }],
    });

    expect(screen.getByTestId("mobile-sidebar-drawer")).toBeInTheDocument();
    expect(screen.queryByTestId("sidebar-swipe-edge")).not.toBeInTheDocument();
  });

  it("closes the mobile drawer when the user swipes it away", () => {
    renderMobileSidebar();

    const edge = screen.getByTestId("sidebar-swipe-edge");
    fireEvent.touchStart(edge, {
      touches: [{ clientX: 8, clientY: 120 }],
    });
    fireEvent.touchMove(edge, {
      touches: [{ clientX: 96, clientY: 126 }],
    });
    fireEvent.touchEnd(edge, {
      changedTouches: [{ clientX: 96, clientY: 126 }],
    });

    const drawer = screen.getByTestId("mobile-sidebar-drawer");
    fireEvent.touchStart(drawer, {
      touches: [{ clientX: 240, clientY: 140 }],
    });
    fireEvent.touchMove(drawer, {
      touches: [{ clientX: 120, clientY: 144 }],
    });
    fireEvent.touchEnd(drawer, {
      changedTouches: [{ clientX: 120, clientY: 144 }],
    });

    expect(screen.queryByTestId("mobile-sidebar-drawer")).not.toBeInTheDocument();
    expect(screen.getByTestId("sidebar-swipe-edge")).toBeInTheDocument();
  });
});
