import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { OrgContextValue } from "../../context/orgContextValue";
import { PENDING_INVITE_STORAGE_KEY } from "../../platform/pendingInvite";
import type { SessionUser } from "../../types/app";
import {
    makeApiError,
    makeMutationState,
    makeOrganisation,
    makeSessionUser,
    makeTeam,
    makeTurnstileControl,
    makeWorkspace,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { OnboardingPage } from "./OnboardingPage";

const { useCreateWorkspace, useForgotPassword, useTurnstile } = vi.hoisted(() => ({
    useCreateWorkspace: vi.fn(),
    useForgotPassword: vi.fn(),
    useTurnstile: vi.fn(),
}));

vi.mock("../../hooks/mutations/organisations", () => ({ useCreateWorkspace }));
vi.mock("../../hooks/mutations/auth", () => ({ useForgotPassword }));
vi.mock("../../hooks/useTurnstile", () => ({ useTurnstile }));

function Landed() {
    const location = useLocation();

    return <p>landed on {location.pathname}</p>;
}

function renderOnboarding(user: SessionUser = makeSessionUser(), org: Partial<OrgContextValue> = {}) {
    const selectOrganisation = vi.fn();

    renderWithProviders(
        <Routes>
            <Route path="/onboarding" element={<OnboardingPage />} />
            <Route path="*" element={<Landed />} />
        </Routes>,
        { route: "/onboarding", user, org: { selectOrganisation, ...org } },
    );

    return { selectOrganisation };
}

function creating(workspace: unknown) {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSuccess?: (created: unknown) => void }) => {
            options?.onSuccess?.(workspace);
        }),
    });
}

async function fillIn(user: ReturnType<typeof userEvent.setup>) {
    await user.type(screen.getByLabelText("Organisation name"), " Harbour ");
    await user.type(screen.getByLabelText("Team name"), " Crew ");
}

beforeEach(() => {
    useCreateWorkspace.mockReturnValue(makeMutationState());
    useForgotPassword.mockReturnValue(makeMutationState());
    useTurnstile.mockReturnValue(makeTurnstileControl());
});

describe("OnboardingPage", () => {
    it("asks for the organisation's name and its first team's name", () => {
        // given
        const user = makeSessionUser();

        // when
        renderOnboarding(user);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Create your workspace" })).toBeInTheDocument();
        expect(
            screen.getByText("Name your organisation and its first team. You can rename both later."),
        ).toBeInTheDocument();
        expect(screen.getByRole("form", { name: "Create your workspace" })).toBeInTheDocument();
        expect(screen.getByLabelText("Organisation name")).toHaveAccessibleDescription("Up to 80 characters.");
        expect(screen.getByLabelText("Team name")).toHaveAccessibleDescription("Your first team. Up to 80 characters.");
        expect(screen.getByRole("button", { name: "Create workspace" })).toBeEnabled();
        expect(document.title).toContain("Create your workspace");
    });

    it("stops both names at the length the server keeps", () => {
        // given
        const user = makeSessionUser();

        // when
        renderOnboarding(user);

        // then
        expect(screen.getByLabelText("Organisation name")).toHaveAttribute("maxlength", "80");
        expect(screen.getByLabelText("Team name")).toHaveAttribute("maxlength", "80");
    });

    it("creates the workspace with the names entered", async () => {
        // given
        const create = makeMutationState();
        useCreateWorkspace.mockReturnValue(create);
        const user = userEvent.setup();
        renderOnboarding();

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create workspace" }));

        // then
        expect(create.mutate).toHaveBeenCalledExactlyOnceWith(
            { name: "Harbour", team_name: "Crew" },
            expect.anything(),
        );
    });

    it("makes the new organisation current and opens its boards, where the first board guide waits", async () => {
        // given
        const workspace = makeWorkspace({
            organisation: makeOrganisation({ id: "org-7", slug: "harbour" }),
            team: makeTeam({ id: "team-3" }),
        });
        useCreateWorkspace.mockReturnValue(creating(workspace));
        const user = userEvent.setup();
        const { selectOrganisation } = renderOnboarding();

        // when
        await fillIn(user);
        await user.click(screen.getByRole("button", { name: "Create workspace" }));

        // then
        expect(selectOrganisation).toHaveBeenCalledExactlyOnceWith("org-7");
        expect(screen.getByText("landed on /o/harbour")).toBeInTheDocument();
    });

    it("says what is missing, ties each message to its field and moves focus to the summary", async () => {
        // given
        const create = makeMutationState();
        useCreateWorkspace.mockReturnValue(create);
        const user = userEvent.setup();
        renderOnboarding();

        // when
        await user.click(screen.getByRole("button", { name: "Create workspace" }));

        // then
        expect(screen.getByLabelText("Organisation name")).toHaveAccessibleDescription(
            "Up to 80 characters. Enter an organisation name.",
        );
        expect(screen.getByLabelText("Team name")).toHaveAccessibleDescription(
            "Your first team. Up to 80 characters. Enter a team name.",
        );
        const summary = screen.getByRole("alert");
        expect(summary).toHaveTextContent("Enter an organisation name. Enter a team name.");
        expect(summary).toHaveFocus();
        expect(create.mutate).not.toHaveBeenCalled();
        expect(create.reset).toHaveBeenCalled();
    });

    it("counts a name of spaces as missing", async () => {
        // given
        const create = makeMutationState();
        useCreateWorkspace.mockReturnValue(create);
        const user = userEvent.setup();
        renderOnboarding();

        // when
        await user.type(screen.getByLabelText("Organisation name"), "Harbour");
        await user.type(screen.getByLabelText("Team name"), "   ");
        await user.click(screen.getByRole("button", { name: "Create workspace" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Enter a team name.");
        expect(screen.getByLabelText("Organisation name")).not.toBeInvalid();
        expect(create.mutate).not.toHaveBeenCalled();
    });

    it("shows the server's reason when the user already owns as many organisations as allowed", () => {
        // given
        const failure = makeApiError({
            status: 409,
            message: "you already own as many organisations as you can",
            code: "owned_limit_reached",
        });
        useCreateWorkspace.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderOnboarding();

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("you already own as many organisations as you can");
        expect(alert).toHaveFocus();
        expect(screen.getByLabelText("Organisation name")).not.toBeInvalid();
        expect(screen.getByLabelText("Team name")).not.toBeInvalid();
    });

    it.each([
        { status: 400, message: "an organisation name is required", code: "org_name_required" },
        { status: 400, message: "an organisation name can be at most 80 characters", code: "org_name_too_long" },
    ])("ties the server's refusal of the organisation name to its field: $code", refusal => {
        // given
        useCreateWorkspace.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        renderOnboarding();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Organisation name")).toHaveAccessibleDescription(
            expect.stringContaining(refusal.message),
        );
        expect(screen.getByLabelText("Team name")).not.toBeInvalid();
    });

    it.each([
        { status: 400, message: "a team name is required", code: "team_name_required" },
        { status: 400, message: "a team name can be at most 80 characters", code: "team_name_too_long" },
    ])("ties the server's refusal of the team name to its field: $code", refusal => {
        // given
        useCreateWorkspace.mockReturnValue(makeMutationState({ isError: true, error: makeApiError(refusal) }));

        // when
        renderOnboarding();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(refusal.message);
        expect(screen.getByLabelText("Team name")).toHaveAccessibleDescription(
            expect.stringContaining(refusal.message),
        );
        expect(screen.getByLabelText("Organisation name")).not.toBeInvalid();
    });

    it("says creating the workspace failed when the failure carries no message", () => {
        // given
        const failure = makeApiError({ status: 500, message: "" });
        useCreateWorkspace.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderOnboarding();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Creating your workspace failed. Try again.");
    });

    it("holds the button while the workspace is being created", () => {
        // given
        useCreateWorkspace.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderOnboarding();

        // then
        expect(screen.getByRole("button", { name: "Create workspace" })).toBeDisabled();
    });
});

describe("OnboardingPage for a user part way through an invite", () => {
    it("sends a user with no organisation to the invite, so they get no workspace of their own", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "invite-1");

        // when
        renderOnboarding(makeSessionUser(), { organisations: [] });

        // then
        expect(screen.getByText("landed on /invite")).toBeInTheDocument();
        expect(screen.queryByRole("form", { name: "Create your workspace" })).not.toBeInTheDocument();
    });

    it("waits for the user's organisations before sending them to the invite", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "invite-1");

        // when
        renderOnboarding(makeSessionUser(), { organisations: [], loading: true });

        // then
        expect(screen.queryByText("landed on /invite")).not.toBeInTheDocument();
    });

    it("lets a user who already has an organisation create another workspace", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "invite-1");

        // when
        renderOnboarding(makeSessionUser(), { organisations: [makeOrganisation()] });

        // then
        expect(screen.getByRole("form", { name: "Create your workspace" })).toBeInTheDocument();
    });
});

describe("OnboardingPage and the account's password", () => {
    it("offers to set a password to an account that has none", () => {
        // given
        const user = makeSessionUser({ email: "sam@gmail.com", has_password: false });

        // when
        renderOnboarding(user);

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Set a password" })).toBeInTheDocument();
        expect(screen.getByText(/Your account has no password/)).toHaveTextContent(
            "We will email a link to sam@gmail.com that lets you set one",
        );
        expect(screen.getByRole("button", { name: "Set a password" })).toBeEnabled();
    });

    it("offers nothing about a password to an account that has one", () => {
        // given
        const user = makeSessionUser({ has_password: true });

        // when
        renderOnboarding(user);

        // then
        expect(screen.queryByRole("heading", { name: "Set a password" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Set a password" })).not.toBeInTheDocument();
    });
});
