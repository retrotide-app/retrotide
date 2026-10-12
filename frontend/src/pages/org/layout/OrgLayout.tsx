import { useEffect, useState } from "react";
import { Navigate, Outlet, useLocation, useParams } from "react-router";
import { ButtonLink } from "../../../components/Button/ButtonLink";
import { PageState } from "../../../components/PageState/PageState";
import {
    findOrganisationBySlug,
    findRenamedOrganisation,
    isNoLongerListed,
    organisationHomePath,
    renamedOrganisationPath,
    type SeenOrganisation,
} from "../../../domain/organisations";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import { usePageTitle } from "../../../hooks/usePageTitle";

const NOT_FOUND_TITLE = "Organisation not found";

function OrganisationNotFound() {
    usePageTitle(NOT_FOUND_TITLE);

    return (
        <PageState
            title={NOT_FOUND_TITLE}
            description="The address may be wrong, or you may not be a member of this organisation."
        >
            <ButtonLink to="/">Go to the home page</ButtonLink>
        </PageState>
    );
}

export function OrgLayout() {
    const { orgSlug = "" } = useParams();
    const { pathname } = useLocation();
    const { organisations, current, selectOrganisation } = useCurrentOrg();
    const [seen, setSeen] = useState<SeenOrganisation | null>(null);

    const routed = findOrganisationBySlug(organisations, orgSlug);
    const renamed = routed === null ? findRenamedOrganisation(organisations, seen, orgSlug) : null;
    const shownId = (routed ?? renamed)?.id ?? null;
    const currentId = current?.id ?? null;

    if (routed !== null && (seen === null || seen.id !== routed.id || seen.slug !== routed.slug)) {
        setSeen({ id: routed.id, slug: routed.slug });
    }

    useEffect(() => {
        if (shownId === null || shownId === currentId) {
            return;
        }

        selectOrganisation(shownId);
    }, [shownId, currentId, selectOrganisation]);

    if (shownId === null && current !== null && isNoLongerListed(organisations, seen, orgSlug)) {
        return <Navigate to={organisationHomePath(current.slug)} replace />;
    }

    if (shownId === null) {
        return <OrganisationNotFound />;
    }

    if (shownId !== currentId) {
        return <div className="loading">Loading...</div>;
    }

    return (
        <>
            {renamed !== null && <Navigate to={renamedOrganisationPath(pathname, renamed.slug)} replace />}
            <Outlet />
        </>
    );
}
