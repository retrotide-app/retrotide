import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeMutationState,
    makeOrganisation,
    makeTeam,
    makeUpgradeRequiredError,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AssignableOrgRole } from "../../../types/api";
import { EmailInviteForm } from "./EmailInviteForm";

const { useCreateInvite } = vi.hoisted(() => ({ useCreateInvite: vi.fn() }));

vi.mock("../../../hooks/mutations/invites", () => ({ useCreateInvite }));

const harbour = makeOrganisation({ id: "org-harbour" });
const crew = makeTeam({ id: "team-crew", name: "Crew" });

interface HarnessProps {
    roles: readonly AssignableOrgRole[];
    onClose: () => void;
}

function Harness({ roles, onClose }: HarnessProps) {
    const [open, setOpen] = useState(true);

    return (
        <EmailInviteForm
            organisation={harbour}
            roles={roles}
            teams={[crew]}
            open={open}
            onClose={() => {
                setOpen(false);
                onClose();
            }}
        />
    );
}

function renderForm(roles: readonly AssignableOrgRole[] = ["admin", "member"]) {
    const onClose = vi.fn();

    renderWithProviders(<Harness roles={roles} onClose={onClose} />);

    return onClose;
}

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    vi.clearAllMocks();
    useCreateInvite.mockReturnValue(makeMutationState());
});

describe("EmailInviteForm", () => {
    it("opens as a dialog that says how long the link works, with the address focused", () => {
        // when
        renderForm();

        // then
        expect(screen.getByRole("dialog", { name: "Invite by email" })).toBeInTheDocument();
        expect(screen.getByText("We email them a link that works for 7 days.")).toBeInTheDocument();
        expect(screen.getByLabelText("Email address")).toHaveFocus();
    });

    it("renders nothing while it is closed", () => {
        // when
        renderWithProviders(
            <EmailInviteForm organisation={harbour} roles={["member"]} teams={[crew]} open={false} onClose={vi.fn()} />,
        );

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("invites the address entered, without its spaces, as a member of no team, and closes", async () => {
        // given
        const invite = makeMutationState({ mutate: vi.fn(succeed) });
        useCreateInvite.mockReturnValue(invite);
        const user = userEvent.setup();
        const onClose = renderForm();

        // when
        await user.type(screen.getByLabelText("Email address"), " kit@example.test ");
        await user.click(screen.getByRole("button", { name: "Send invite" }));

        // then
        expect(invite.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", email: "kit@example.test", role: "member" },
            expect.anything(),
        );
        expect(onClose).toHaveBeenCalledOnce();
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("invites with the role and team chosen", async () => {
        // given
        const invite = makeMutationState();
        useCreateInvite.mockReturnValue(invite);
        const user = userEvent.setup();
        renderForm();

        // when
        await user.type(screen.getByLabelText("Email address"), "kit@example.test");
        await user.selectOptions(screen.getByLabelText("Role"), "admin");
        await user.selectOptions(screen.getByLabelText("Team"), "team-crew");
        await user.click(screen.getByRole("button", { name: "Send invite" }));

        // then
        expect(invite.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", email: "kit@example.test", role: "admin", team_id: "team-crew" },
            expect.anything(),
        );
    });

    it("offers only the roles the viewer may invite", () => {
        // when
        renderForm(["member"]);

        // then
        expect(screen.getByLabelText("Role")).toHaveDisplayValue("Member");
        expect(screen.queryByRole("option", { name: "Admin" })).not.toBeInTheDocument();
    });

    it("closes without inviting anyone when cancelled", async () => {
        // given
        const invite = makeMutationState();
        useCreateInvite.mockReturnValue(invite);
        const user = userEvent.setup();
        const onClose = renderForm();

        // when
        await user.type(screen.getByLabelText("Email address"), "kit@example.test");
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(onClose).toHaveBeenCalledOnce();
        expect(invite.mutate).not.toHaveBeenCalled();
        expect(invite.reset).toHaveBeenCalled();
    });

    it("says what is missing, ties it to the field and moves focus to it, without inviting anyone", async () => {
        // given
        const invite = makeMutationState();
        useCreateInvite.mockReturnValue(invite);
        const user = userEvent.setup();
        renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Send invite" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter an email address.");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(screen.getByLabelText("Email address")).toBeInvalid();
        expect(invite.mutate).not.toHaveBeenCalled();
        expect(invite.reset).toHaveBeenCalled();
    });

    it("says who the invite was sent to once the dialog closes", async () => {
        // given
        const invite = makeMutationState({
            mutate: vi.fn((variables: unknown, options?: { onSuccess?: () => void }) => {
                useCreateInvite.mockReturnValue(makeMutationState({ isSuccess: true }));
                succeed(variables, options);
            }),
        });
        useCreateInvite.mockReturnValue(invite);
        const user = userEvent.setup();
        renderForm();

        // when
        await user.type(screen.getByLabelText("Email address"), "kit@example.test");
        await user.click(screen.getByRole("button", { name: "Send invite" }));

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Invite sent to kit@example.test.");
    });

    it("holds the button while the invite is sent", () => {
        // given
        useCreateInvite.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderForm();

        // then
        expect(screen.getByRole("button", { name: "Send invite" })).toBeDisabled();
    });

    it.each([
        { status: 400, message: "a valid email address is required" },
        { status: 409, message: "already a member of this organisation", code: "already_member" },
        { status: 409, message: "that email address already has a pending invite", code: "invite_pending" },
    ])("ties the server's refusal of the address to the field: $message", refusal => {
        // given
        useCreateInvite.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        renderForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(screen.getByLabelText("Email address")).toHaveAccessibleDescription(
            expect.stringContaining(refusal.message),
        );
    });

    it.each([
        { status: 503, message: "email is not available on this site" },
        { status: 500, message: "the invite was saved, but its email was not sent, so resend it" },
        { status: 403, message: "inviting an admin needs the permission to make admins" },
        { status: 409, message: "that team is archived" },
        { status: 404, message: "team not found" },
    ])("shows any other refusal as it comes, without blaming the address: $message", refusal => {
        // given
        useCreateInvite.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        renderForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Email address")).not.toHaveAttribute("aria-invalid");
    });

    it("ties the refusal of an archived team to the team field, not the address", () => {
        // given
        const error = makeApiError({ status: 409, message: "that team is archived", code: "team_archived" });
        useCreateInvite.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("that team is archived");
        expect(screen.getByLabelText("Team")).toHaveAccessibleDescription(
            expect.stringContaining("that team is archived"),
        );
        expect(screen.getByLabelText("Email address")).not.toHaveAttribute("aria-invalid");
    });

    it("says paid plans are coming when the plan allows no more members", () => {
        // given
        const error = makeUpgradeRequiredError({ limit: "members", allowed: 10, current: 10 });
        useCreateInvite.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("The free plan allows 10 members.");
        expect(screen.getByRole("alert")).toHaveTextContent("Paid plans are coming.");
    });
});
