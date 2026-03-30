import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FormRenderer } from "./form-renderer";
import { useFormStore } from "@/lib/stores/form-store";

const submitFormMock = jest.fn();

jest.mock("next-intl", () => ({
  useTranslations: () => (key: string, values?: Record<string, string>) => {
    if (key === "submitForm") {
      return "Submit form";
    }
    if (key === "createdTask") {
      return `Created task ${values?.id ?? ""}`;
    }
    return key;
  },
}));

describe("FormRenderer", () => {
  beforeEach(() => {
    submitFormMock.mockReset();
    submitFormMock.mockResolvedValue({ id: "task-123" });

    useFormStore.setState({
      submitForm: submitFormMock,
    });
  });

  it("renders form fields, submits values, and shows the created task id", async () => {
    const user = userEvent.setup();

    render(
      <FormRenderer
        form={{
          id: "form-1",
          projectId: "project-1",
          name: "Bug intake",
          slug: "bug-intake",
          fields: [
            { key: "title", label: "Title" },
            { key: "severity", label: "Severity" },
          ],
          targetStatus: "todo",
          isPublic: false,
          createdAt: "",
          updatedAt: "",
        }}
      />,
    );

    const [titleInput, severityInput] = screen.getAllByRole("textbox");
    await user.type(titleInput, "Crash on launch");
    await user.type(severityInput, "P1");
    await user.click(screen.getByRole("button", { name: "Submit form" }));

    await waitFor(() => {
      expect(submitFormMock).toHaveBeenCalledWith("bug-intake", {
        values: {
          title: "Crash on launch",
          severity: "P1",
        },
      });
    });
    expect(screen.getByText("Created task task-123")).toBeInTheDocument();
  });
});
