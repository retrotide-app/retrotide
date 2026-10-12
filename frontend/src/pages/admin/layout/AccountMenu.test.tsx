import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeSessionUser } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { AccountMenu } from "./AccountMenu";

const { useLogout } = vi.hoisted(() => ({ useLogout: vi.fn() }));

vi.mock("../../../hooks/mutations/auth", () => ({ useLogout }));

const sam = makeSessionUser({ display_name: "Sam Rivers", email: "sam@example.test" });

function renderMenu() {
    return renderWithProviders(
        <Routes>
            <Route
                path="/admin"
                element={
                    <>
                        <AccountMenu backTo="/o/harbour/teams" />
                        <p>the page</p>
                    </>
                }
            />
            <Route path="/login" element={<p>the sign-in page</p>} />
        </Routes>,
        { user: sam, auth: { siteRole: "admin" }, route: "/admin" },
    );
}

function trigger() {
    return screen.getByRole("button", { name: "Sam Rivers" });
}

beforeEach(() => {
    useLogout.mockReturnValue(makeMutationState());
});

describe("AccountMenu", () => {
    it("names the signed-in staff member on its button and starts closed", () => {
        // when
        renderMenu();

        // then
        expect(trigger()).toHaveAttribute("aria-expanded", "false");
        expect(trigger()).toHaveTextContent("SR");
        expect(screen.queryByRole("button", { name: "Sign out" })).not.toBeInTheDocument();
    });

    it("shows the first letter of the email address when the account has no display name", () => {
        // given
        const nameless = makeSessionUser({ display_name: "", email: "robin@example.test" });

        // when
        renderWithProviders(<AccountMenu backTo="/o/harbour/teams" />, {
            user: nameless,
            auth: { siteRole: "admin" },
            route: "/admin",
        });

        // then the button is the avatar alone, named by the email address
        const trigger = screen.getByRole("button", { name: "robin@example.test" });
        expect(trigger).toHaveTextContent(/^R$/);
    });

    it("shows who is signed in, their role, the theme and the way out when opened", async () => {
        // given
        const user = userEvent.setup();
        renderMenu();

        // when
        await user.click(trigger());

        // then
        expect(trigger()).toHaveAttribute("aria-expanded", "true");
        expect(screen.getByText("sam@example.test")).toBeInTheDocument();
        expect(screen.getByText("Admin")).toBeInTheDocument();
        expect(screen.getByRole("combobox", { name: "Theme" })).toBeInTheDocument();
        const links = within(screen.getByRole("navigation", { name: "Account" }));
        expect(links.getByRole("link", { name: "Back to app" })).toHaveAttribute("href", "/o/harbour/teams");
        expect(links.getByRole("link", { name: "Your account" })).toHaveAttribute("href", "/account");
        expect(screen.getByRole("button", { name: "Sign out" })).toBeInTheDocument();
    });

    it("closes with the escape key and hands focus back to its button", async () => {
        // given
        const user = userEvent.setup();
        renderMenu();
        await user.click(trigger());

        // when
        await user.keyboard("{Escape}");

        // then
        expect(trigger()).toHaveAttribute("aria-expanded", "false");
        expect(trigger()).toHaveFocus();
    });

    it("closes when something outside it is pressed", async () => {
        // given
        const user = userEvent.setup();
        renderMenu();
        await user.click(trigger());

        // when
        await user.click(screen.getByText("the page"));

        // then
        expect(trigger()).toHaveAttribute("aria-expanded", "false");
    });

    it("signs out and goes to the sign-in page", async () => {
        // given
        const user = userEvent.setup();
        useLogout.mockReturnValue(
            makeMutationState({
                mutate: vi.fn((_variables: unknown, options?: { onSuccess?: () => void }) => {
                    options?.onSuccess?.();
                }),
            }),
        );
        renderMenu();
        await user.click(trigger());

        // when
        await user.click(screen.getByRole("button", { name: "Sign out" }));

        // then
        expect(screen.getByText("the sign-in page")).toBeInTheDocument();
    });

    it("says why signing out failed", () => {
        // given
        useLogout.mockReturnValue(
            makeMutationState({ isError: true, error: makeApiError({ status: 500, message: "failed to sign out" }) }),
        );

        // when
        renderMenu();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("failed to sign out");
    });
});
