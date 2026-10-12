import { ButtonLink } from "../../../components/Button/ButtonLink";
import { PageState } from "../../../components/PageState/PageState";
import { organisationPath } from "../../../domain/organisations";
import { usePageTitle } from "../../../hooks/usePageTitle";
import type { OrganisationResponse } from "../../../types/api";
import { formatExactDateTime } from "../../../utils/time";

interface DeletedOrganisationProps {
    organisation: OrganisationResponse;
    pageTitle: string;
}

export function DeletedOrganisation({ organisation, pageTitle }: DeletedOrganisationProps) {
    usePageTitle(pageTitle);

    const deadline = organisation.restorable_until;
    const until = deadline === undefined ? "" : ` until ${formatExactDateTime(deadline)}`;

    return (
        <PageState
            title={`${organisation.name} is deleted`}
            description={`You can restore it${until} from the organisation settings.`}
        >
            <ButtonLink to={organisationPath(organisation.slug, "settings")} variant="primary">
                Go to organisation settings
            </ButtonLink>
        </PageState>
    );
}
