import { ButtonLink } from "../../../components/Button/ButtonLink";
import { SettingsRow } from "../../../components/Settings/SettingsRow";
import { useOrganisationExportUrl } from "../../../hooks/queries/exports";
import type { OrganisationResponse } from "../../../types/api";
import styles from "../shared/OrgSection.module.css";

interface OrganisationExportSectionProps {
    organisation: OrganisationResponse;
}

export function OrganisationExportSection({ organisation }: OrganisationExportSectionProps) {
    const exportUrl = useOrganisationExportUrl(organisation.id);

    return (
        <SettingsRow
            title="Export organisation"
            headingLevel={2}
            description="Download every board in the organisation as a JSON file. Each board is exported as you see it, so cards that are still hidden stay hidden."
        >
            <div className={styles.actions}>
                <ButtonLink href={exportUrl} download>
                    Download export
                </ButtonLink>
            </div>
        </SettingsRow>
    );
}
