import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeCreatedInvite,
    makeMutationState,
    makeOrganisation,
    makeTeam,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AssignableOrgRole } from "../../../types/api";
import { LinkInviteForm } from "./LinkInviteForm";

const { useCreateInvite, copyText } = vi.hoisted(() => ({ useCreateInvite: vi.fn(), copyText: vi.fn() }));

vi.mock("../../../hooks/mutations/invites", () => ({ useCreateInvite }));
vi.mock("../../../platform/clipboard", () => ({ copyText }));

const harbour = makeOrganisation({ id: "org-harbour" });
const crew = makeTeam({ id: "team-crew", name: "Crew" });
const link = "https://retrotide.test/invite#token-abc";

function renderForm(roles: readonly AssignableOrgRole[] = ["admin", "member"], open = true) {
    const onClose = vi.fn();

    renderWithProviders(
        <LinkInviteForm organisation={harbour} roles={roles} teams={[crew]} open={open} onClose={onClose} />,
    );

    return onClose;
}

function showCreated(url: string | undefined) {
    useCreateInvite.mockReturnValue(
        makeMutationState({ isSuccess: true, data: makeCreatedInvite({ email: undefined, max_uses: undefined, url }) }),
    );
}

beforeEach(() => {
    vi.clearAllMocks();
    copyText.mockResolvedValue(undefined);
    useCreateInvite.mockReturnValue(makeMutationState());
});

describe("LinkInviteForm", () => {
    it("opens as a dialog that says who the link works for", () => {
        // when
        renderForm();

        // then
        expect(screen.getByRole("dialog", { name: "Invite with a link" })).toBeInTheDocument();
        expect(screen.getByText("Anyone who has the link can join until it expires in 7 days.")).toBeInTheDocument();
    });

    it("renders nothing while it is closed", () => {
        // when
        renderForm(["member"], false);

        // then
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });

    it("creates a link for a member of no team", async () => {
        // given
        const creation = makeMutationState();
        useCreateInvite.mockReturnValue(creation);
        const user = userEvent.setup();
        renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Create link" }));

        // then
        expect(creation.mutate).toHaveBeenCalledExactlyOnceWith({ orgId: "org-harbour", role: "member" });
    });

    it("creates a link with the role and team chosen", async () => {
        // given
        const creation = makeMutationState();
        useCreateInvite.mockReturnValue(creation);
        const user = userEvent.setup();
        renderForm();

        // when
        await user.selectOptions(screen.getByLabelText("Role"), "admin");
        await user.selectOptions(screen.getByLabelText("Team"), "team-crew");
        await user.click(screen.getByRole("button", { name: "Create link" }));

        // then
        expect(creation.mutate).toHaveBeenCalledExactlyOnceWith({
            orgId: "org-harbour",
            role: "admin",
            team_id: "team-crew",
        });
    });

    it("closes without creating anything when cancelled", async () => {
        // given
        const creation = makeMutationState();
        useCreateInvite.mockReturnValue(creation);
        const user = userEvent.setup();
        const onClose = renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Cancel" }));

        // then
        expect(onClose).toHaveBeenCalledOnce();
        expect(creation.mutate).not.toHaveBeenCalled();
    });

    it("shows the new link once, and says to copy it now", () => {
        // given
        showCreated(link);

        // when
        renderForm();

        // then
        expect(screen.getByRole("dialog", { name: "Invite link ready" })).toBeInTheDocument();
        expect(screen.getByLabelText("Invite link")).toHaveValue(link);
        expect(screen.getByRole("status")).toHaveTextContent(
            "Invite link created. Copy it now, because it is shown only once.",
        );
    });

    it("copies the link and says so", async () => {
        // given
        showCreated(link);
        const user = userEvent.setup();
        renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Copy link" }));

        // then
        expect(copyText).toHaveBeenCalledExactlyOnceWith(link);
        expect(await screen.findByText("Link copied.")).toBeInTheDocument();
    });

    it("forgets the link and closes when done", async () => {
        // given
        const creation = makeMutationState({
            isSuccess: true,
            data: makeCreatedInvite({ email: undefined, max_uses: undefined, url: link }),
        });
        useCreateInvite.mockReturnValue(creation);
        const user = userEvent.setup();
        const onClose = renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Done" }));

        // then
        expect(creation.reset).toHaveBeenCalled();
        expect(onClose).toHaveBeenCalledOnce();
    });

    it("shows the browser's reason when the link could not be copied", async () => {
        // given
        copyText.mockRejectedValue(new Error("Clipboard access was denied"));
        showCreated(link);
        const user = userEvent.setup();
        renderForm();

        // when
        await user.click(screen.getByRole("button", { name: "Copy link" }));

        // then
        expect(await screen.findByRole("alert")).toHaveTextContent("Clipboard access was denied");
        expect(screen.queryByText("Link copied.")).not.toBeInTheDocument();
    });

    it("says so when the invite was created but its link did not come back", () => {
        // given
        showCreated(undefined);

        // when
        renderForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "The invite was created, but the answer did not include its link.",
        );
        expect(screen.queryByLabelText("Invite link")).not.toBeInTheDocument();
    });

    it("holds the button while the link is created", () => {
        // given
        useCreateInvite.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderForm();

        // then
        expect(screen.getByRole("button", { name: "Create link" })).toBeDisabled();
    });

    it("shows the server's refusal as it comes and moves focus to it", () => {
        // given
        const error = makeApiError({ status: 403, message: "inviting an admin needs the permission to make admins" });
        useCreateInvite.mockReturnValue(makeMutationState({ isError: true, error }));

        // when
        renderForm();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("inviting an admin needs the permission to make admins");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(screen.queryByLabelText("Invite link")).not.toBeInTheDocument();
        expect(screen.getByLabelText("Team")).not.toHaveAttribute("aria-invalid");
    });

    it("ties the refusal of an archived team to the team field", () => {
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
    });
});
