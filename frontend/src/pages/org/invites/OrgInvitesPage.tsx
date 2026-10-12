import { useState } from "react";
import { Button } from "../../../components/Button/Button";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { invitableRoles, orgRoleGrants } from "../../../domain/organisationRoles";
import { isArchived, isDeleted } from "../../../domain/organisations";
import { useTeams } from "../../../hooks/queries/teams";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import type { OrganisationResponse, OrgRole } from "../../../types/api";
import { DeletedOrganisation } from "../shared/DeletedOrganisation";
import { OrgPage } from "../shared/OrgPage";
import sectionStyles from "../shared/OrgSection.module.css";
import { EmailInviteForm } from "./EmailInviteForm";
import { InviteList } from "./InviteList";
import { LinkInviteForm } from "./LinkInviteForm";
import styles from "./OrgInvitesPage.module.css";

const PAGE_TITLE = "Invites";

type InviteKind = "email" | "link";

interface InvitesProps {
    organisation: OrganisationResponse;
    role: OrgRole | null;
}

function Invites({ organisation, role }: InvitesProps) {
    const teams = useTeams(organisation.id);
    const { email_enabled } = useSiteInfo();
    const [inviting, setInviting] = useState<InviteKind | null>(null);
    const roles = invitableRoles(role);
    const listed = teams.data?.teams ?? [];
    const live = listed.filter(team => !isArchived(team));

    return (
        <div className={sectionStyles.stack}>
            {teams.error !== null && <LoadFailure message={teams.error.message} onRetry={teams.refetch} />}
            <div className={styles.toolbar}>
                <p className={styles.intro}>
                    {email_enabled
                        ? "Invite someone by email, or create a link you can share yourself. Both work for 7 days."
                        : "Invites are off until this site can send email. A site admin can set up email in the admin console."}
                </p>
                <div className={styles.buttons}>
                    <Button type="button" disabled={!email_enabled} onClick={() => setInviting("link")}>
                        Create invite link
                    </Button>
                    <Button
                        type="button"
                        variant="primary"
                        disabled={!email_enabled}
                        onClick={() => setInviting("email")}
                    >
                        Invite by email
                    </Button>
                </div>
            </div>
            <EmailInviteForm
                organisation={organisation}
                roles={roles}
                teams={live}
                open={inviting === "email"}
                onClose={() => setInviting(null)}
            />
            <LinkInviteForm
                organisation={organisation}
                roles={roles}
                teams={live}
                open={inviting === "link"}
                onClose={() => setInviting(null)}
            />
            <InviteList organisation={organisation} role={role} teams={listed} />
        </div>
    );
}

export function OrgInvitesPage() {
    const { current, role } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    if (!orgRoleGrants(role, "org.manage_members")) {
        return (
            <OrgPage title={PAGE_TITLE}>
                <p>Only the owner and admins can invite people.</p>
            </OrgPage>
        );
    }

    return (
        <OrgPage
            title={PAGE_TITLE}
            description="Bring people into your organisation by email or with a link, and choose the team they join. Invites last 7 days."
        >
            <Invites key={current.id} organisation={current} role={role} />
        </OrgPage>
    );
}
