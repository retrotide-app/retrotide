import { useState } from "react";
import { Button } from "../../components/Button/Button";
import { ConfirmDialog } from "../../components/ConfirmDialog/ConfirmDialog";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import { Toast } from "../../components/Toast/Toast";
import { canLeaveOrganisation } from "../../domain/organisationRoles";
import { isDeleted } from "../../domain/organisations";
import { useLeaveOrganisation } from "../../hooks/mutations/organisations";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import type { OrganisationResponse } from "../../types/api";
import { ActionFailure } from "../org/shared/ActionFailure";
import styles from "./AccountSection.module.css";

export function LeaveOrganisationSection() {
    const { organisations } = useCurrentOrg();
    const leave = useLeaveOrganisation();
    const [leaving, setLeaving] = useState<OrganisationResponse | null>(null);
    const [leftName, setLeftName] = useState("");

    const leavable = organisations.filter(
        organisation => !isDeleted(organisation) && canLeaveOrganisation(organisation.role ?? null),
    );

    if (leavable.length === 0) {
        return null;
    }

    function handleLeave() {
        if (leaving === null) {
            return;
        }

        setLeftName(leaving.name);
        leave.mutate({ orgId: leaving.id }, { onSuccess: () => setLeaving(null) });
    }

    const body = (
        <div className={styles.form}>
            <p>
                You lose access to <bdi>{leaving?.name}</bdi> and its teams and boards. To come back, you need a new
                invite.
            </p>
            {leave.error !== null && <ActionFailure error={leave.error} takeFocus />}
        </div>
    );

    return (
        <SettingsRow
            title="Organisations"
            headingLevel={2}
            description="Organisations you belong to. Leaving one removes your access to it."
        >
            <ul className={styles.list}>
                {leavable.map(organisation => (
                    <li key={organisation.id} className={styles.item}>
                        <span className={styles.itemName}>
                            <bdi>{organisation.name}</bdi>
                        </span>
                        <Button
                            type="button"
                            size="small"
                            onClick={() => {
                                leave.reset();
                                setLeaving(organisation);
                            }}
                        >
                            Leave
                        </Button>
                    </li>
                ))}
            </ul>
            <ConfirmDialog
                open={leaving !== null}
                title={`Leave ${leaving?.name ?? ""}?`}
                body={body}
                confirmLabel="Leave organisation"
                destructive
                busy={leave.isPending}
                onConfirm={handleLeave}
                onCancel={() => {
                    leave.reset();
                    setLeaving(null);
                }}
            />
            {leave.isSuccess && (
                <Toast variant="success" onDismiss={leave.reset}>
                    You left <bdi>{leftName}</bdi>.
                </Toast>
            )}
        </SettingsRow>
    );
}
