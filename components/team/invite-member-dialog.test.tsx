import userEvent from "@testing-library/user-event";
import { render, screen, waitFor } from "@testing-library/react";
import { InviteMemberDialog } from "./invite-member-dialog";
import type { InvitationCreateResponse } from "@/lib/stores/invitation-store";

describe("InviteMemberDialog", () => {
  const createdResponse: InvitationCreateResponse = {
    invitation: {
      id: "inv-1",
      projectId: "project-1",
      inviterUserId: "user-1",
      invitedIdentity: { kind: "email", value: "agent@example.com" },
      projectRole: "editor",
      status: "pending",
      expiresAt: new Date().toISOString(),
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    acceptToken: "plaintext",
    acceptUrl: "http://localhost:3000/invitations/accept?token=plaintext",
  };

  it("submits an email invitation and shows the accept URL", async () => {
    const user = userEvent.setup();
    const onInvite = jest.fn().mockResolvedValue(createdResponse);
    const onOpenChange = jest.fn();

    render(
      <InviteMemberDialog
        open={true}
        onOpenChange={onOpenChange}
        onInvite={onInvite}
      />,
    );

    await user.type(
      screen.getByPlaceholderText("member@example.com"),
      "agent@example.com",
    );
    await user.click(screen.getByRole("button", { name: /send invitation/i }));

    await waitFor(() =>
      expect(onInvite).toHaveBeenCalledWith({
        invitedIdentity: { kind: "email", value: "agent@example.com" },
        projectRole: "editor",
        message: undefined,
      }),
    );
    await waitFor(() =>
      expect(
        screen.getByDisplayValue(createdResponse.acceptUrl),
      ).toBeInTheDocument(),
    );
  });

  it("requires email or IM identity before submitting", async () => {
    const user = userEvent.setup();
    const onInvite = jest.fn();

    render(
      <InviteMemberDialog
        open={true}
        onOpenChange={() => undefined}
        onInvite={onInvite}
      />,
    );

    await user.click(screen.getByRole("button", { name: /send invitation/i }));
    expect(onInvite).not.toHaveBeenCalled();
    expect(screen.getByText(/email is required/i)).toBeInTheDocument();
  });
});
