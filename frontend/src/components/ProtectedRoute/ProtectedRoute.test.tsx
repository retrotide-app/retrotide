import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router";
import { describe, expect, it, vi } from "vitest";
import type { StaffPermission } from "../../domain/permissions";
import { makeApiError, makeSessionUser } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { SessionUser } from "../../types/app";
import { ProtectedRoute } from "./ProtectedRoute";

interface ProtectedOptions {
    user?: SessionUser | null;
    loading?: boolean;
    error?: Error | null;
    retry?: () => Promise<unknown>;
    permission?: StaffPermission;
    permissions?: StaffPermission[];
    route?: string;
}

function LoginProbe() {
    const location = useLocation();
    const state = location.state as { from?: string } | null;

    return <p>{`sign in first, then back to ${state?.from ?? "nowhere"}`}</p>;
}

function renderProtected(options: ProtectedOptions = {}) {
    return renderWithProviders(
        <Routes>
            <Route element={<ProtectedRoute permission={options.permission} />}>
                <Route path="/board" element={<p>the team board</p>} />
            </Route>
            <Route path="/login" element={<LoginProbe />} />
            <Route path="/" element={<p>front page</p>} />
        </Routes>,
        {
            route: options.route ?? "/board",
            user: options.user ?? null,
            permissions: options.permissions ?? [],
            auth: {
                loading: options.loading ?? false,
                error: options.error ?? null,
                ...(options.retry ? { retry: options.retry } : {}),
            },
        },
    );
}

describe("ProtectedRoute while the session is still unknown", () => {
    it("holds the route with a loading placeholder", () => {
        // given
        const loading = true;

        // when
        renderProtected({ loading });

        // then
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByText("the team board")).not.toBeInTheDocument();
    });

    it("does not send a visitor to the login page before the session has resolved", () => {
        // given
        const loading = true;

        // when
        renderProtected({ loading, user: null });

        // then
        expect(screen.queryByText(/sign in first/)).not.toBeInTheDocument();
    });

    it("holds the route even for a member whose session is still resolving", () => {
        // given
        const user = makeSessionUser();

        // when
        renderProtected({ loading: true, user });

        // then
        expect(screen.getByText("Loading...")).toBeInTheDocument();
        expect(screen.queryByText("the team board")).not.toBeInTheDocument();
    });
});

describe("ProtectedRoute when the session could not be checked", () => {
    it("says so with the server's message instead of treating the visitor as signed out", () => {
        // given
        const error = makeApiError({ status: 503, message: "service unavailable" });

        // when
        renderProtected({ error });

        // then
        expect(screen.getByRole("heading", { level: 1, name: "We could not check your sign-in" })).toBeInTheDocument();
        expect(screen.getByRole("alert")).toHaveTextContent("service unavailable");
        expect(screen.queryByText(/sign in first/)).not.toBeInTheDocument();
        expect(screen.queryByText("the team board")).not.toBeInTheDocument();
    });

    it("checks again when asked", async () => {
        // given
        const retry = vi.fn().mockResolvedValue(undefined);
        const user = userEvent.setup();
        renderProtected({ error: makeApiError({ status: 503, message: "service unavailable" }), retry });

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(retry).toHaveBeenCalledOnce();
    });

    it("keeps showing the page to a member whose session was known before the check failed", () => {
        // given
        const user = makeSessionUser();

        // when
        renderProtected({ user, error: makeApiError({ status: 503, message: "service unavailable" }) });

        // then
        expect(screen.getByText("the team board")).toBeInTheDocument();
    });
});

describe("ProtectedRoute for a signed out visitor", () => {
    it("sends the visitor to the login page with the page to come back to", () => {
        // given
        const route = "/board?column=wins";

        // when
        renderProtected({ route });

        // then
        expect(screen.getByText("sign in first, then back to /board?column=wins")).toBeInTheDocument();
    });

    it("never reveals the protected content", () => {
        // given
        const user = null;

        // when
        renderProtected({ user });

        // then
        expect(screen.queryByText("the team board")).not.toBeInTheDocument();
        expect(screen.queryByText("front page")).not.toBeInTheDocument();
    });
});

describe("ProtectedRoute for a signed in member", () => {
    it("shows the protected content", () => {
        // given
        const user = makeSessionUser();

        // when
        renderProtected({ user });

        // then
        expect(screen.getByText("the team board")).toBeInTheDocument();
        expect(screen.queryByText("Loading...")).not.toBeInTheDocument();
    });

    it("leaves a signed in member where they are rather than on the login page", () => {
        // given
        const user = makeSessionUser();

        // when
        renderProtected({ user });

        // then
        expect(screen.queryByText(/sign in first/)).not.toBeInTheDocument();
        expect(screen.queryByText("front page")).not.toBeInTheDocument();
    });

    it("draws its children in place of the outlet when it is given some", () => {
        // given
        const ui = (
            <ProtectedRoute>
                <p>wrapped content</p>
            </ProtectedRoute>
        );

        // when
        renderWithProviders(ui, { user: makeSessionUser() });

        // then
        expect(screen.getByText("wrapped content")).toBeInTheDocument();
    });
});

describe("ProtectedRoute with a staff permission", () => {
    it("shows the page to staff who hold the permission", () => {
        // given
        const options = {
            user: makeSessionUser(),
            permission: "view_users" as const,
            permissions: ["view_users" as const],
        };

        // when
        renderProtected(options);

        // then
        expect(screen.getByText("the team board")).toBeInTheDocument();
    });

    it("tells a member without the permission they cannot open the page", () => {
        // given
        const options = { user: makeSessionUser(), permission: "manage_settings" as const, permissions: [] };

        // when
        renderProtected(options);

        // then
        expect(screen.getByRole("heading", { name: "You cannot open this page" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Go to the home page" })).toHaveAttribute("href", "/");
        expect(screen.queryByText("the team board")).not.toBeInTheDocument();
    });

    it("heads the refusal one level down when it guards part of a page that has its own heading", () => {
        // given
        const ui = (
            <ProtectedRoute permission="manage_settings" headingLevel={2}>
                <p>the settings</p>
            </ProtectedRoute>
        );

        // when
        renderWithProviders(ui, { user: makeSessionUser(), permissions: [] });

        // then
        expect(screen.getByRole("heading", { level: 2, name: "You cannot open this page" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
    });

    it("still sends a signed out visitor to sign in rather than refusing them", () => {
        // given
        const options = { user: null, permission: "view_users" as const };

        // when
        renderProtected(options);

        // then
        expect(screen.getByText(/sign in first/)).toBeInTheDocument();
    });
});
