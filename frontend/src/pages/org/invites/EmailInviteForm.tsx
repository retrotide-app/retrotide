import { useState } from "react";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../../components/FormDialog/FormDialog";
import { TextField } from "../../../components/FormField/TextField";
import { Toast } from "../../../components/Toast/Toast";
import { fieldRefusal, INVITE_TEAM_REFUSALS } from "../../../domain/refusals";
import { useCreateInvite } from "../../../hooks/mutations/invites";
import type { AssignableOrgRole, OrganisationResponse, TeamResponse } from "../../../types/api";
import { errorCode, errorStatus } from "../../../utils/errorMessage";
import { ActionFailure } from "../shared/ActionFailure";
import { InviteOptions } from "./InviteOptions";
import styles from "./OrgInvitesPage.module.css";

interface EmailInviteFormProps {
    organisation: OrganisationResponse;
    roles: readonly AssignableOrgRole[];
    teams: readonly TeamResponse[];
    open: boolean;
    onClose: () => void;
}

const EMAIL_MISSING = "Enter an email address.";

const DEFAULT_ROLE: AssignableOrgRole = "member";

const ADDRESS_REFUSAL_CODES: ReadonlySet<string> = new Set(["already_member", "invite_pending"]);

function addressRefusal(error: Error | null): string | null {
    if (error === null) {
        return null;
    }

    if (errorStatus(error) === 400 || ADDRESS_REFUSAL_CODES.has(errorCode(error) ?? "")) {
        return error.message;
    }

    return null;
}

export function EmailInviteForm({ organisation, roles, teams, open, onClose }: EmailInviteFormProps) {
    const invite = useCreateInvite();
    const [email, setEmail] = useState("");
    const [wantedRole, setWantedRole] = useState<AssignableOrgRole>(DEFAULT_ROLE);
    const [wantedTeam, setWantedTeam] = useState("");
    const [problem, setProblem] = useState<string | null>(null);
    const [sentTo, setSentTo] = useState("");

    const role = roles.includes(wantedRole) ? wantedRole : DEFAULT_ROLE;
    const teamId = teams.some(team => team.id === wantedTeam) ? wantedTeam : "";
    const fieldError = problem ?? addressRefusal(invite.error);
    const teamError = fieldRefusal(invite.error, INVITE_TEAM_REFUSALS);

    function handleCancel() {
        setEmail("");
        setProblem(null);
        invite.reset();
        onClose();
    }

    function handleSubmit() {
        const address = email.trim();

        if (address === "") {
            invite.reset();
            setProblem(EMAIL_MISSING);
            return;
        }

        setProblem(null);
        setSentTo(address);
        invite.mutate(
            { orgId: organisation.id, email: address, role, ...(teamId === "" ? {} : { team_id: teamId }) },
            {
                onSuccess: () => {
                    setEmail("");
                    onClose();
                },
            },
        );
    }

    return (
        <>
            <FormDialog
                open={open}
                title="Invite by email"
                submitLabel="Send invite"
                busy={invite.isPending}
                onSubmit={handleSubmit}
                onCancel={handleCancel}
            >
                <p className={styles.note}>We email them a link that works for 7 days.</p>
                {problem !== null && <ErrorBanner message={problem} takeFocus />}
                {problem === null && invite.error !== null && <ActionFailure error={invite.error} takeFocus />}
                <TextField
                    label="Email address"
                    type="email"
                    value={email}
                    onChange={e => setEmail(e.target.value)}
                    error={fieldError}
                    autoComplete="off"
                    required
                />
                <InviteOptions
                    roles={roles}
                    role={role}
                    onRoleChange={setWantedRole}
                    teams={teams}
                    teamId={teamId}
                    onTeamChange={setWantedTeam}
                    teamError={teamError}
                />
            </FormDialog>
            {invite.isSuccess && !open && (
                <Toast variant="success" onDismiss={invite.reset}>
                    Invite sent to <bdi>{sentTo}</bdi>.
                </Toast>
            )}
        </>
    );
}
