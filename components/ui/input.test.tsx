import { fireEvent, render, screen } from "@testing-library/react";
import { Input } from "./input";

describe("Input", () => {
  it("renders an input element with the expected base attributes", () => {
    render(<Input type="email" placeholder="Email" disabled />);

    const input = screen.getByPlaceholderText("Email");
    expect(input).toHaveAttribute("data-slot", "input");
    expect(input).toHaveAttribute("type", "email");
    expect(input).toBeDisabled();
    expect(input).toHaveClass("border-input");
  });

  it("forwards change handlers and merges custom class names", () => {
    const handleChange = jest.fn();
    render(
      <Input
        aria-label="Name"
        className="bg-test"
        onChange={handleChange}
      />,
    );

    const input = screen.getByRole("textbox", { name: "Name" });
    fireEvent.change(input, { target: { value: "Alice" } });

    expect(handleChange).toHaveBeenCalledTimes(1);
    expect(input).toHaveClass("bg-test");
  });
});
