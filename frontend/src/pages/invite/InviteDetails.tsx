import type { InviteDetailsResponse, OrgRole } from "../../types/api";
import { formatExactDateTime } from "../../utils/time";
import styles from "./InviteAcceptancePage.module.css";

interface InviteDetailsProps {
    invite: InviteDetailsResponse;
}

const ROLE_PHRASES: Readonly<Record<OrgRole, string>> = {
    owner: "an owner",
    admin: "an admin",
    member: "a member",
};

export function InviteDetails({ invite }: InviteDetailsProps) {
    const inviter = invite.inviter_name ?? "";
    const team = invite.team_name ?? "";
    const expiry = formatExactDateTime(invite.expires_at);

    return (
        <div className={styles.details}>
            <p>
                {inviter === "" ? (
                    "You are invited to join "
                ) : (
                    <>
                        <bdi>{inviter}</bdi> invited you to join{" "}
                    </>
                )}
                <bdi>{invite.org_name}</bdi> as {ROLE_PHRASES[invite.role]}.
            </p>
            {team !== "" && (
                <p>
                    You will also join the team <bdi>{team}</bdi>.
                </p>
            )}
            {expiry !== "" && <p>The invite expires on {expiry}.</p>}
        </div>
    );
}
