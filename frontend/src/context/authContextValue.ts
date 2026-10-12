import { createContext } from "react";
import type { StaffPermission } from "../domain/permissions";
import type { StaffRole } from "../types/api";
import type { SessionUser } from "../types/app";

export interface AuthContextValue {
    user: SessionUser | null;
    guestId: string | null;
    permissions: readonly StaffPermission[];
    siteRole: StaffRole | null;
    loading: boolean;
    error: Error | null;
    retry: () => Promise<unknown>;
}

export const AuthContext = createContext<AuthContextValue | null>(null);
