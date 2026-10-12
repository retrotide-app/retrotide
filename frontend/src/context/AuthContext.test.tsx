import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { isUserSessionHeld, setUserSessionHeld } from "../api/sessionLost";
import { useAuth } from "../hooks/useAuth";
import { makeApiError, makeQueryState, makeSession, makeSessionUser } from "../test-utils/fixtures";
import { renderWithProviders } from "../test-utils/render";
import { FakeWebSocket } from "../test-utils/ws";
import { AuthProvider } from "./AuthContext";

const { useSession } = vi.hoisted(() => ({ useSession: vi.fn() }));

vi.mock("../hooks/queries/auth", () => ({ useSession }));

function Probe() {
    const { user, guestId, permissions, siteRole, loading, error } = useAuth();

    return (
        <div>
            <p>{`user: ${user?.display_name ?? "nobody"}`}</p>
            <p>{`guest: ${guestId ?? "none"}`}</p>
            <p>{`permissions: ${permissions.join(", ") || "none"}`}</p>
            <p>{`site role: ${siteRole ?? "none"}`}</p>
            <p>{`loading: ${String(loading)}`}</p>
            <p>{`error: ${error?.message ?? "none"}`}</p>
        </div>
    );
}

function renderProvider() {
    return renderWithProviders(
        <AuthProvider>
            <Probe />
        </AuthProvider>,
        { user: makeSessionUser({ display_name: "Outer Harness" }), auth: { loading: true, siteRole: "admin" } },
    );
}

beforeEach(() => {
    FakeWebSocket.instances = [];
    vi.stubGlobal("WebSocket", FakeWebSocket);
    useSession.mockReturnValue(makeQueryState({ data: { authenticated: false } }));
});

describe("AuthProvider for a signed out visitor", () => {
    it("reports nobody signed in, not the harness's user", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: { authenticated: false } }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("user: nobody")).toBeInTheDocument();
        expect(screen.getByText("permissions: none")).toBeInTheDocument();
        expect(screen.getByText("site role: none")).toBeInTheDocument();
        expect(screen.getByText("loading: false")).toBeInTheDocument();
    });

    it("reports no site role even when a stale answer still carries one", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: { authenticated: false, site_role: "admin" } }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("site role: none")).toBeInTheDocument();
    });

    it("tells the transport that no user session is held, so a 401 cannot mark the session lost", () => {
        // given
        setUserSessionHeld(true);

        // when
        renderProvider();

        // then
        expect(isUserSessionHeld()).toBe(false);
    });

    it("opens no socket", () => {
        // when
        renderProvider();

        // then
        expect(FakeWebSocket.instances).toHaveLength(0);
    });
});

describe("AuthProvider for a signed in user", () => {
    it("reports the session's user and staff permissions", () => {
        // given
        const session = makeSession({
            user: makeSessionUser({ display_name: "Robin" }),
            permissions: ["view_admin_panel", "view_users"],
        });
        useSession.mockReturnValue(makeQueryState({ data: session }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("user: Robin")).toBeInTheDocument();
        expect(screen.getByText("permissions: view_admin_panel, view_users")).toBeInTheDocument();
    });

    it("drops a permission the catalogue does not know", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: makeSession({ permissions: ["view_users", "fly"] }) }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("permissions: view_users")).toBeInTheDocument();
    });

    it("grants no permissions when the session carries none", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: makeSession() }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("permissions: none")).toBeInTheDocument();
    });

    it("reports the viewer's site staff role", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: makeSession({ site_role: "support" }) }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("site role: support")).toBeInTheDocument();
    });

    it("reports no site role for a viewer who holds no staff role", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: makeSession() }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("site role: none")).toBeInTheDocument();
    });

    it("tells the transport a user session is held", () => {
        // given
        setUserSessionHeld(false);
        useSession.mockReturnValue(makeQueryState({ data: makeSession() }));

        // when
        renderProvider();

        // then
        expect(isUserSessionHeld()).toBe(true);
    });

    it("opens one realtime socket for the signed in user, so their boards update live", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: makeSession() }));

        // when
        renderProvider();

        // then
        expect(FakeWebSocket.instances).toHaveLength(1);
        expect(FakeWebSocket.instances[0].url).toBe("ws://localhost:3000/api/v1/ws");
    });

    it("keeps the one socket when the session is read again for the same user", () => {
        // given
        const session = makeSession({ user: makeSessionUser({ id: "u1", display_name: "Robin" }) });
        useSession.mockReturnValue(makeQueryState({ data: session }));
        const { rerender } = renderProvider();

        // when
        useSession.mockReturnValue(
            makeQueryState({ data: { ...session, user: makeSessionUser({ id: "u1", display_name: "Robin" }) } }),
        );
        rerender(
            <AuthProvider>
                <Probe />
            </AuthProvider>,
        );

        // then
        expect(FakeWebSocket.instances).toHaveLength(1);
        expect(FakeWebSocket.instances[0].closeCount).toBe(0);
    });

    it("closes the socket when the user signs out", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: makeSession() }));
        const { rerender } = renderProvider();

        // when
        useSession.mockReturnValue(makeQueryState({ data: { authenticated: false } }));
        rerender(
            <AuthProvider>
                <Probe />
            </AuthProvider>,
        );

        // then
        expect(FakeWebSocket.instances[0].closeCount).toBe(1);
    });

    it("closes the socket when the provider unmounts", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: makeSession() }));
        const { unmount } = renderProvider();

        // when
        unmount();

        // then
        expect(FakeWebSocket.instances[0].closeCount).toBe(1);
    });
});

describe("AuthProvider for a guest", () => {
    const guestSession = { authenticated: false, guest: { id: "g1" } };

    it("reports the guest and nobody signed in", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: guestSession }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("user: nobody")).toBeInTheDocument();
        expect(screen.getByText("guest: g1")).toBeInTheDocument();
        expect(screen.getByText("permissions: none")).toBeInTheDocument();
    });

    it("holds no user session, so a guest's 401 never reads as a lost session", () => {
        // given
        setUserSessionHeld(true);
        useSession.mockReturnValue(makeQueryState({ data: guestSession }));

        // when
        renderProvider();

        // then
        expect(isUserSessionHeld()).toBe(false);
    });

    it("opens the realtime socket for the guest, so the board they joined updates live", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: guestSession }));

        // when
        renderProvider();

        // then
        expect(FakeWebSocket.instances).toHaveLength(1);
    });

    it("replaces the guest's socket with the user's once the guest signs in", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: guestSession }));
        const { rerender } = renderProvider();

        // when
        useSession.mockReturnValue(makeQueryState({ data: makeSession() }));
        rerender(
            <AuthProvider>
                <Probe />
            </AuthProvider>,
        );

        // then
        expect(FakeWebSocket.instances).toHaveLength(2);
        expect(FakeWebSocket.instances[0].closeCount).toBe(1);
        expect(screen.getByText("guest: none")).toBeInTheDocument();
    });

    it("prefers the user when a stale answer carries both", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ data: { ...makeSession(), guest: { id: "g1" } } }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("guest: none")).toBeInTheDocument();
        expect(screen.getByText("user: Sam")).toBeInTheDocument();
    });
});

describe("AuthProvider while the session is unknown", () => {
    it("reports loading until the session has been checked", () => {
        // given
        useSession.mockReturnValue(makeQueryState({ loading: true }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("loading: true")).toBeInTheDocument();
        expect(screen.getByText("user: nobody")).toBeInTheDocument();
    });

    it("reports the error when the session could not be checked, rather than claiming nobody is signed in", () => {
        // given
        const failure = makeApiError({ status: 503, message: "service unavailable" });
        useSession.mockReturnValue(makeQueryState({ error: failure }));

        // when
        renderProvider();

        // then
        expect(screen.getByText("error: service unavailable")).toBeInTheDocument();
        expect(screen.getByText("loading: false")).toBeInTheDocument();
    });
});
