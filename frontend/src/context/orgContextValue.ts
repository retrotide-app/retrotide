import { createContext } from "react";
import type { OrganisationResponse, OrgRole } from "../types/api";

export interface OrgContextValue {
    organisations: readonly OrganisationResponse[];
    current: OrganisationResponse | null;
    role: OrgRole | null;
    loading: boolean;
    error: Error | null;
    retry: () => Promise<unknown>;
    selectOrganisation: (orgId: string) => void;
}

export const OrgContext = createContext<OrgContextValue | null>(null);
