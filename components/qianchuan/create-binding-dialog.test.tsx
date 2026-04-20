jest.mock("@/lib/stores/qianchuan-bindings-store", () => ({
  useQianchuanBindingsStore: jest.fn(),
}));

jest.mock("@/lib/stores/secrets-store", () => ({
  useSecretsStore: jest.fn(),
}));

import { render, screen } from "@testing-library/react";
import { CreateBindingDialog } from "./create-binding-dialog";
import { useQianchuanBindingsStore } from "@/lib/stores/qianchuan-bindings-store";
import { useSecretsStore } from "@/lib/stores/secrets-store";

type Selector<S, R> = (state: S) => R;

describe("CreateBindingDialog", () => {
  const createBinding = jest.fn().mockResolvedValue({ id: "b1" });
  const fetchSecrets = jest.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    jest.clearAllMocks();

    (useQianchuanBindingsStore as unknown as jest.Mock).mockImplementation(
      <S, R>(sel: Selector<S, R>) =>
        sel({ createBinding, byProject: {}, loading: {} } as unknown as S),
    );
    (useSecretsStore as unknown as jest.Mock).mockImplementation(
      <S, R>(sel: Selector<S, R>) =>
        sel({
          secretsByProject: {
            p1: [{ name: "qc.A1.access" }, { name: "qc.A1.refresh" }],
          },
          fetchSecrets,
        } as unknown as S),
    );
  });

  it("renders the manual-token notice and the OAuth-deferred hint", () => {
    render(
      <CreateBindingDialog projectId="p1" open onOpenChange={() => {}} />,
    );
    expect(
      screen.getByText("新增千川绑定（手动 token）"),
    ).toBeInTheDocument();
    expect(screen.getByText(/Plan 3B/)).toBeInTheDocument();
  });

  it("triggers fetchSecrets on open", () => {
    render(
      <CreateBindingDialog projectId="p1" open onOpenChange={() => {}} />,
    );
    expect(fetchSecrets).toHaveBeenCalledWith("p1");
  });
});
