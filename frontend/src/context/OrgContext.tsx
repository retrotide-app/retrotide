import { type PropsWithChildren, useCallback, useMemo, useState } from "react";
import { chooseCurrentOrganisation } from "../domain/organisations";
import { useOrganisations } from "../hooks/queries/organisations";
import { useAuth } from "../hooks/useAuth";
import { readLastOrganisation, writeLastOrganisation } from "../platform/lastOrganisation";
import type { OrganisationResponse } from "../types/api";
import { OrgContext, type OrgContextValue } from "./orgContextValue";

const NO_ORGANISATIONS: readonly OrganisationResponse[] = [];

export function OrgProvider({ children }: PropsWithChildren) {
    const { user } = useAuth();
    const signedIn = user !== null;
    const { data, loading, error, refetch } = useOrganisations(signedIn);
    const [chosenId, setChosenId] = useState<string | null>(readLastOrganisation);

    const organisations = signedIn ? (data?.organisations ?? NO_ORGANISATIONS) : NO_ORGANISATIONS;
    const current = useMemo(() => chooseCurrentOrganisation(organisations, chosenId), [organisations, chosenId]);

    const selectOrganisation = useCallback((orgId: string) => {
        setChosenId(orgId);
        writeLastOrganisation(orgId);
    }, []);

    const value = useMemo<OrgContextValue>(
        () => ({
            organisations,
            current,
            role: current?.role ?? null,
            loading: signedIn && loading,
            error: signedIn ? error : null,
            retry: refetch,
            selectOrganisation,
        }),
        [organisations, current, signedIn, loading, error, refetch, selectOrganisation],
    );

    return <OrgContext.Provider value={value}>{children}</OrgContext.Provider>;
}
