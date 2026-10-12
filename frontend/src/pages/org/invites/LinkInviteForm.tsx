import { useState } from "react";
import { Banner } from "../../../components/Banner/Banner";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../../components/FormDialog/FormDialog";
import { TextField } from "../../../components/FormField/TextField";
import { Modal } from "../../../components/Modal/Modal";
import { fieldRefusal, INVITE_TEAM_REFUSALS } from "../../../domain/refusals";
import { useCreateInvite } from "../../../hooks/mutations/invites";
import { useCopyText } from "../../../hooks/useCopyText";
import type { AssignableOrgRole, OrganisationResponse, TeamResponse } from "../../../types/api";
import { ActionFailure } from "../shared/ActionFailure";
import { InviteOptions } from "./InviteOptions";
import styles from "./OrgInvitesPage.module.css";

interface LinkInviteFormProps {
    organisation: OrganisationResponse;
    roles: readonly AssignableOrgRole[];
    teams: readonly TeamResponse[];
    open: boolean;
    onClose: () => void;
}

const DEFAULT_ROLE: AssignableOrgRole = "member";

const LINK_MISSING = "The invite was created, but the answer did not include its link.";

export function LinkInviteForm({ organisation, roles, teams, open, onClose }: LinkInviteFormProps) {
    const creation = useCreateInvite();
    const copy = useCopyText();
    const [wantedRole, setWantedRole] = useState<AssignableOrgRole>(DEFAULT_ROLE);
    const [wantedTeam, setWantedTeam] = useState("");

    const role = roles.includes(wantedRole) ? wantedRole : DEFAULT_ROLE;
    const teamId = teams.some(team => team.id === wantedTeam) ? wantedTeam : "";
    const created = creation.isSuccess ? creation.data : undefined;
    const url = created?.url;
    const teamError = fieldRefusal(creation.error, INVITE_TEAM_REFUSALS);

    function close() {
        creation.reset();
        copy.reset();
        onClose();
    }

    function handleSubmit() {
        copy.reset();
        creation.mutate({ orgId: organisation.id, role, ...(teamId === "" ? {} : { team_id: teamId }) });
    }

    if (url !== undefined) {
        return (
            <Modal isOpen={open} onClose={close} title="Invite link ready">
                <div className={styles.dialog}>
                    <Banner tone="success" role="status">
                        Invite link created. Copy it now, because it is shown only once.
                    </Banner>
                    <TextField label="Invite link" value={url} readOnly onFocus={e => e.target.select()} />
                    {copy.copied && (
                        <Banner tone="success" role="status">
                            Link copied.
                        </Banner>
                    )}
                    {copy.error !== null && <ErrorBanner message={copy.error.message} takeFocus />}
                    <div className={styles.dialogActions}>
                        <Button type="button" variant="secondary" onClick={close}>
                            Done
                        </Button>
                        <Button type="button" variant="primary" onClick={() => copy.copy(url)}>
                            Copy link
                        </Button>
                    </div>
                </div>
            </Modal>
        );
    }

    return (
        <FormDialog
            open={open}
            title="Invite with a link"
            submitLabel="Create link"
            busy={creation.isPending}
            onSubmit={handleSubmit}
            onCancel={close}
        >
            <p className={styles.note}>Anyone who has the link can join until it expires in 7 days.</p>
            {creation.error !== null && <ActionFailure error={creation.error} takeFocus />}
            {created !== undefined && <ErrorBanner message={LINK_MISSING} takeFocus />}
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
    );
}
