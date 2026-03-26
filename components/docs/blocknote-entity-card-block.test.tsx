import { render, screen } from "@testing-library/react";

jest.mock("next/link", () => ({
  __esModule: true,
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: React.ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

jest.mock("@blocknote/core", () => ({
  defaultProps: {
    textAlignment: { default: "left" },
  },
}));

jest.mock("@blocknote/react", () => ({
  BlockContentWrapper: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="block-content-wrapper">{children}</div>
  ),
  createReactBlockSpec: jest.fn(
    (
      config: unknown,
      implementation: {
        render: (input: {
          block: {
            props?: { entityType?: string; entityId?: string; label?: string };
          };
        }) => React.ReactElement;
      },
    ) => ({
      config,
      render: implementation.render,
    }),
  ),
}));

import { createEntityCardBlock } from "./blocknote-entity-card-block";

describe("createEntityCardBlock", () => {
  it("declares the entity card block spec and routes each entity type correctly", () => {
    const entityCardBlock = createEntityCardBlock as unknown as {
      config: unknown;
      render: (input: {
        block: {
          props?: { entityType?: string; entityId?: string; label?: string };
        };
      }) => React.ReactElement;
    };

    expect(entityCardBlock.config).toEqual(
      expect.objectContaining({
        type: "entityCard",
        content: "none",
      }),
    );

    const { rerender } = render(
      entityCardBlock.render({
        block: {
          props: { entityType: "agent", entityId: "agent-9", label: "Planner" },
        },
      }),
    );

    expect(screen.getByRole("link", { name: /Planner/i })).toHaveAttribute(
      "href",
      "/agents?id=agent-9",
    );
    expect(screen.getByText("agent")).toBeInTheDocument();

    rerender(
      entityCardBlock.render({
        block: {
          props: { entityType: "review", entityId: "review-2", label: "Review gate" },
        },
      }),
    );
    expect(screen.getByRole("link", { name: /Review gate/i })).toHaveAttribute(
      "href",
      "/reviews?id=review-2",
    );

    rerender(
      entityCardBlock.render({
        block: {
          props: { entityType: "task", entityId: "task-7", label: "Delivery task" },
        },
      }),
    );
    expect(screen.getByRole("link", { name: /Delivery task/i })).toHaveAttribute(
      "href",
      "/project?id=task-7",
    );
  });
});
