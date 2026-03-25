import userEvent from "@testing-library/user-event";
import { render, screen, waitFor } from "@testing-library/react";
import { InviteMemberDialog } from "./invite-member-dialog";

describe("InviteMemberDialog", () => {
  it("submits member invitations and resets the dialog state", async () => {
    const user = userEvent.setup();
    const onInvite = jest.fn().mockResolvedValue(undefined);
    const onOpenChange = jest.fn();

    render(
      <InviteMemberDialog open={true} onOpenChange={onOpenChange} onInvite={onInvite} />,
    );

    await user.type(screen.getByPlaceholderText("member@example.com"), "agent@example.com");
    await user.selectOptions(screen.getByDisplayValue("Human"), "agent");
    await user.selectOptions(screen.getByDisplayValue("Developer"), "reviewer");
    await user.click(screen.getByRole("button", { name: "Add Member" }));

    await waitFor(() =>
      expect(onInvite).toHaveBeenCalledWith({
        email: "agent@example.com",
        role: "reviewer",
        type: "agent",
      }),
    );
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
