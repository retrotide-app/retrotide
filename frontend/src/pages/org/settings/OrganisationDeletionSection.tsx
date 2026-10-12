import { useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { SettingsRow } from "../../../components/Settings/SettingsRow";
import { Toast } from "../../../components/Toast/Toast";
import { isDeleted } from "../../../domain/organisations";
import { useDeleteOrganisation, useRestoreOrganisation } from "../../../hooks/mutations/organisations";
import type { OrganisationResponse } from "../../../types/api";
import { formatExactDateTime } from "../../../utils/time";
import { ActionFailure } from "../shared/ActionFailure";
import styles from "../shared/OrgSection.module.css";

interface OrganisationDeletionSectionProps {
    organisation: OrganisationResponse;
}

export function OrganisationDeletionSection({ organisation }: OrganisationDeletionSectionProps) {
    const deletion = useDeleteOrganisation();
    const restoration = useRestoreOrganisation();
    const [confirming, setConfirming] = useState(false);

    const deleted = isDeleted(organisation);
    const deadline = organisation.restorable_until;
    const name = <bdi>{organisation.name}</bdi>;

    function openConfirm() {
        deletion.reset();
        restoration.reset();
        setConfirming(true);
    }

    function closeConfirm() {
        deletion.reset();
        setConfirming(false);
    }

    function handleDelete() {
        deletion.mutate(
            { orgId: organisation.id },
            {
                onSuccess: () => {
                    setConfirming(false);
                },
            },
        );
    }

    function handleRestore() {
        deletion.reset();
        restoration.mutate({ orgId: organisation.id });
    }

    const deleteBody = (
        <div className={styles.dialog}>
            <p>
                Every member loses access to {name} and its teams. You can restore it from this page for a limited time.
            </p>
            {deletion.error !== null && <ActionFailure error={deletion.error} takeFocus />}
        </div>
    );

    return (
        <>
            {deleted ? (
                <SettingsRow
                    title="Restore organisation"
                    headingLevel={2}
                    description={
                        <>
                            {name} was deleted on {formatExactDateTime(organisation.deleted_at)}.
                            {deadline !== undefined && ` You can restore it until ${formatExactDateTime(deadline)}.`}
                        </>
                    }
                >
                    {restoration.error !== null && <ActionFailure error={restoration.error} takeFocus />}
                    <div className={styles.actions}>
                        <Button
                            type="button"
                            variant="primary"
                            disabled={restoration.isPending}
                            onClick={handleRestore}
                        >
                            Restore organisation
                        </Button>
                    </div>
                </SettingsRow>
            ) : (
                <SettingsRow
                    title="Delete organisation"
                    headingLevel={2}
                    tone="danger"
                    description={
                        <>
                            Deleting hides {name} and its teams from every member. You can restore it for a limited
                            time.
                        </>
                    }
                >
                    <div className={styles.actions}>
                        <Button type="button" variant="danger" onClick={openConfirm}>
                            Delete organisation
                        </Button>
                    </div>
                </SettingsRow>
            )}

            <ConfirmDialog
                open={confirming && !deleted}
                title={`Delete ${organisation.name}?`}
                body={deleteBody}
                confirmLabel="Delete organisation"
                destructive
                busy={deletion.isPending}
                onConfirm={handleDelete}
                onCancel={closeConfirm}
            />

            {deletion.isSuccess && (
                <Toast variant="success" onDismiss={deletion.reset}>
                    {name} deleted.
                </Toast>
            )}

            {restoration.isSuccess && (
                <Toast variant="success" onDismiss={restoration.reset}>
                    {name} restored.
                </Toast>
            )}
        </>
    );
}
