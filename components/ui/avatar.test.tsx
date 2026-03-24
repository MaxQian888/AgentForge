import { render, screen } from "@testing-library/react";
import {
  Avatar,
  AvatarBadge,
  AvatarFallback,
  AvatarGroup,
  AvatarGroupCount,
  AvatarImage,
} from "./avatar";

describe("Avatar", () => {
  it("renders image, fallback, badge, and group affordances", () => {
    const { container } = render(
      <AvatarGroup>
        <Avatar size="lg" className="ring-primary">
          <AvatarImage src="/avatar.png" alt="Taylor" />
          <AvatarFallback>TA</AvatarFallback>
          <AvatarBadge>•</AvatarBadge>
        </Avatar>
        <AvatarGroupCount>+2</AvatarGroupCount>
      </AvatarGroup>
    );

    const avatar = container.querySelector('[data-slot="avatar"]');
    expect(avatar).toHaveAttribute("data-size", "lg");
    expect(avatar).toHaveClass("ring-primary");
    expect(screen.getByText("TA")).toHaveAttribute("data-slot", "avatar-fallback");
    expect(screen.getByText("•")).toHaveAttribute("data-slot", "avatar-badge");
    expect(container.querySelector('[data-slot="avatar-group"]')).toBeInTheDocument();
    expect(screen.getByText("+2")).toHaveAttribute("data-slot", "avatar-group-count");
  });
});
