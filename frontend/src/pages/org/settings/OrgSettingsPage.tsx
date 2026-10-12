import { SettingsGroup } from "../../../components/Settings/SettingsGroup";
import { orgRoleGrants } from "../../../domain/organisationRoles";
import { isDeleted } from "../../../domain/organisations";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import { OrgPage } from "../shared/OrgPage";
import { GuestAccessSection } from "./GuestAccessSection";
import { OrganisationAddressSection } from "./OrganisationAddressSection";
import { OrganisationDeletionSection } from "./OrganisationDeletionSection";
import { OrganisationExportSection } from "./OrganisationExportSection";
import { OrganisationNameSection } from "./OrganisationNameSection";

const PAGE_TITLE = "Organisation settings";

export function OrgSettingsPage() {
    const { current, role } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    const deleted = isDeleted(current);
    const owns = orgRoleGrants(role, "org.delete");
    const exports = orgRoleGrants(role, "org.export");

    if (!orgRoleGrants(role, "org.update")) {
        return (
            <OrgPage title={PAGE_TITLE}>
                <p>Only the owner and admins can change these settings.</p>
            </OrgPage>
        );
    }

    return (
        <OrgPage
            title={PAGE_TITLE}
            description="Your organisation's name and web address, whether guests can join boards, exporting its boards, and deleting the organisation."
        >
            <SettingsGroup key={current.id}>
                {!deleted && <OrganisationNameSection organisation={current} />}
                {!deleted && <OrganisationAddressSection organisation={current} />}
                {!deleted && <GuestAccessSection organisation={current} />}
                {!deleted && exports && <OrganisationExportSection organisation={current} />}
                {owns && <OrganisationDeletionSection organisation={current} />}
            </SettingsGroup>
        </OrgPage>
    );
}
