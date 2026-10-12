import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeSessionUser } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { Header } from "./Header";

const { useLogout } = vi.hoisted(() => ({ useLogout: vi.fn() }));

vi.mock("../../../hooks/mutations/auth", () => ({ useLogout }));

function signingOutSucceeds() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSuccess?: () => void }) => {
            options?.onSuccess?.();
        }),
    });
}

beforeEach(() => {
    useLogout.mockReturnValue(makeMutationState());
});

describe("Header", () => {
    it("asks its owner to toggle the sidebar when the menu button is pressed", async () => {
        // given
        const onToggleSidebar = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Header onToggleSidebar={onToggleSidebar} sidebarOpen={false} />);

        // when
        await user.click(screen.getByRole("button", { name: "Toggle menu" }));

        // then
        expect(onToggleSidebar).toHaveBeenCalledOnce();
    });

    it("reports the sidebar as collapsed while it is closed", () => {
        // given
        const sidebarOpen = false;

        // when
        renderWithProviders(<Header onToggleSidebar={vi.fn()} sidebarOpen={sidebarOpen} />);

        // then
        expect(screen.getByRole("button", { name: "Toggle menu" })).toHaveAttribute("aria-expanded", "false");
    });

    it("reports the sidebar as expanded while it is open", () => {
        // given
        const sidebarOpen = true;

        // when
        renderWithProviders(<Header onToggleSidebar={vi.fn()} sidebarOpen={sidebarOpen} />);

        // then
        expect(screen.getByRole("button", { name: "Toggle menu" })).toHaveAttribute("aria-expanded", "true");
    });

    it("points the menu button at the sidebar it controls", () => {
        // given
        const ui = <Header onToggleSidebar={vi.fn()} sidebarOpen={false} />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("button", { name: "Toggle menu" })).toHaveAttribute("aria-controls", "app-sidebar");
    });

    it("keeps the theme select on show", () => {
        // given
        const ui = <Header onToggleSidebar={vi.fn()} sidebarOpen={false} />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("combobox", { name: "Theme" })).toBeInTheDocument();
    });

    it("renders as the page banner", () => {
        // given
        const ui = <Header onToggleSidebar={vi.fn()} sidebarOpen={false} />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("banner")).toContainElement(screen.getByRole("button", { name: "Toggle menu" }));
    });
});

describe("Header account actions", () => {
    it("offers a signed out visitor the way to sign in", () => {
        // given
        const visitor = null;

        // when
        renderWithProviders(<Header onToggleSidebar={vi.fn()} sidebarOpen={false} />, { user: visitor });

        // then
        expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login");
        expect(screen.queryByRole("button", { name: "Sign out" })).not.toBeInTheDocument();
    });

    it("offers a signed in user the way to sign out", () => {
        // given
        const user = makeSessionUser();

        // when
        renderWithProviders(<Header onToggleSidebar={vi.fn()} sidebarOpen={false} />, { user });

        // then
        expect(screen.getByRole("button", { name: "Sign out" })).toBeEnabled();
        expect(screen.queryByRole("link", { name: "Sign in" })).not.toBeInTheDocument();
    });

    it("offers neither while the session is still being checked", () => {
        // given
        const auth = { loading: true };

        // when
        renderWithProviders(<Header onToggleSidebar={vi.fn()} sidebarOpen={false} />, { auth });

        // then
        expect(screen.queryByRole("link", { name: "Sign in" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Sign out" })).not.toBeInTheDocument();
    });

    it("offers neither when the session could not be checked, rather than claiming nobody is signed in", () => {
        // given
        const auth = { error: makeApiError({ status: 503, message: "service unavailable" }) };

        // when
        renderWithProviders(<Header onToggleSidebar={vi.fn()} sidebarOpen={false} />, { auth });

        // then
        expect(screen.queryByRole("link", { name: "Sign in" })).not.toBeInTheDocument();
    });

    it("signs out and moves to the sign in page", async () => {
        // given
        const logout = signingOutSucceeds();
        useLogout.mockReturnValue(logout);
        const user = userEvent.setup();
        renderWithProviders(
            <Routes>
                <Route path="/account" element={<Header onToggleSidebar={vi.fn()} sidebarOpen={false} />} />
                <Route path="/login" element={<p>the sign in page</p>} />
            </Routes>,
            { user: makeSessionUser(), route: "/account" },
        );

        // when
        await user.click(screen.getByRole("button", { name: "Sign out" }));

        // then
        expect(logout.mutate).toHaveBeenCalledOnce();
        expect(screen.getByText("the sign in page")).toBeInTheDocument();
    });

    it("holds the button while signing out", () => {
        // given
        useLogout.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<Header onToggleSidebar={vi.fn()} sidebarOpen={false} />, { user: makeSessionUser() });

        // then
        expect(screen.getByRole("button", { name: "Sign out" })).toBeDisabled();
    });

    it("shows why signing out failed", () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to sign out" });
        useLogout.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<Header onToggleSidebar={vi.fn()} sidebarOpen={false} />, { user: makeSessionUser() });

        // then
        expect(screen.getByRole("status")).toHaveTextContent("failed to sign out");
    });
});
