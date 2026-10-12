import { type PropsWithChildren, useEffect, useMemo } from "react";
import { RealtimeSync } from "../api/realtime/sync/RealtimeSync";
import { setUserSessionHeld } from "../api/sessionLost";
import { staffPermissions } from "../domain/permissions";
import { useSession } from "../hooks/queries/auth";
import type { Viewer } from "../types/app";
import { AuthContext, type AuthContextValue } from "./authContextValue";

export function AuthProvider({ children }: PropsWithChildren) {
    const { data, loading, error, refetch } = useSession();
    const user = data?.authenticated && data.user ? data.user : null;
    const guestId = user === null ? (data?.guest?.id ?? null) : null;
    const granted = data?.permissions;
    const userSessionHeld = user !== null;
    const userId = user?.id ?? null;

    useEffect(() => {
        setUserSessionHeld(userSessionHeld);
    }, [userSessionHeld]);

    const permissions = useMemo(() => (user === null ? [] : staffPermissions(granted ?? [])), [user, granted]);
    const siteRole = user === null ? null : (data?.site_role ?? null);

    const value = useMemo<AuthContextValue>(
        () => ({ user, guestId, permissions, siteRole, loading, error, retry: refetch }),
        [user, guestId, permissions, siteRole, loading, error, refetch],
    );

    const viewer = useMemo<Viewer | null>(() => {
        if (userId !== null) {
            return { kind: "user", id: userId };
        }

        if (guestId !== null) {
            return { kind: "guest", id: guestId };
        }

        return null;
    }, [userId, guestId]);

    return (
        <AuthContext.Provider value={value}>
            <RealtimeSync viewer={viewer} />
            {children}
        </AuthContext.Provider>
    );
}
