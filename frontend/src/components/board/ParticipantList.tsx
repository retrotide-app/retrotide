import { useState } from "react";
import { facilitationRefusal, type ParticipantRow, participantRows } from "../../domain/board/facilitation";
import { initialsOf } from "../../domain/displayName";
import { useRemoveParticipant, useSetParticipantRole } from "../../hooks/mutations/boards";
import type { BoardRole, BoardSnapshot, ParticipantView } from "../../types/api";
import { ConfirmDialog } from "../ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { Menu, type MenuItem } from "../Menu/Menu";
import { Toast } from "../Toast/Toast";
import styles from "./ParticipantList.module.css";

interface ParticipantListProps {
    snapshot: BoardSnapshot;
    plain?: boolean;
    rows?: readonly ParticipantRow[];
}

const ROLE_LABELS: Readonly<Record<BoardRole, string>> = {
    facilitator: "Facilitator",
    participant: "Participant",
};

const ROLE_CHANGED: Readonly<Record<BoardRole, string>> = {
    facilitator: "is now a co-facilitator.",
    participant: "is now a participant.",
};

export function ParticipantList({ snapshot, plain = false, rows: shownRows }: ParticipantListProps) {
    const changeRole = useSetParticipantRole(snapshot.board_id);
    const removal = useRemoveParticipant(snapshot.board_id);
    const [removing, setRemoving] = useState<ParticipantView | null>(null);
    const [removedName, setRemovedName] = useState("");

    const rows = shownRows ?? participantRows(snapshot);
    const changed = changeRole.variables;
    const changedName =
        snapshot.participants.find(participant => participant.participant_id === changed?.participantId)
            ?.display_name ?? "";

    function menuItems({ participant }: ParticipantRow): MenuItem[] {
        const role: BoardRole = participant.role === "facilitator" ? "participant" : "facilitator";

        return [
            {
                label: role === "facilitator" ? "Make co-facilitator" : "Make participant",
                onSelect: () => {
                    changeRole.mutate({ participantId: participant.participant_id, role });
                },
            },
            {
                label: "Remove from board",
                tone: "danger",
                onSelect: () => {
                    removal.reset();
                    setRemoving(participant);
                },
            },
        ];
    }

    function handleRemove() {
        if (removing === null) {
            return;
        }

        setRemovedName(removing.display_name);
        removal.mutate(
            { participantId: removing.participant_id },
            {
                onSuccess: () => {
                    setRemoving(null);
                },
            },
        );
    }

    const removeBody = (
        <div className={styles.dialog}>
            <p>
                <bdi>{removing?.display_name}</bdi> loses access to this board straight away. Their cards stay on the
                board.
            </p>
            {removal.error !== null && <ErrorBanner message={facilitationRefusal(removal.error)} takeFocus />}
        </div>
    );

    return (
        <>
            {rows.length === 0 && <p className={styles.empty}>Nobody has joined this board yet.</p>}
            {rows.length > 0 && (
                <ul
                    className={plain ? `${styles.list} ${styles.plain}` : styles.list}
                    aria-label="People on this board"
                >
                    {rows.map(row => {
                        const { participant, you, manageable } = row;

                        return (
                            <li key={participant.participant_id} className={styles.row}>
                                <span className={styles.avatar} aria-hidden="true">
                                    {initialsOf(participant.display_name)}
                                </span>
                                <span className={styles.who}>
                                    <span className={styles.name}>
                                        <bdi>{participant.display_name}</bdi>
                                        {you && " (you)"}
                                    </span>
                                    <span className={styles.role}>{ROLE_LABELS[participant.role]}</span>
                                </span>
                                <span className={styles.actions}>
                                    {manageable && (
                                        <Menu
                                            label={`Actions for ${participant.display_name}`}
                                            items={menuItems(row)}
                                            iconOnly
                                            size="small"
                                        />
                                    )}
                                </span>
                            </li>
                        );
                    })}
                </ul>
            )}
            <ConfirmDialog
                open={removing !== null}
                title={`Remove ${removing?.display_name ?? ""} from the board?`}
                body={removeBody}
                confirmLabel="Remove"
                destructive
                busy={removal.isPending}
                onConfirm={handleRemove}
                onCancel={() => {
                    removal.reset();
                    setRemoving(null);
                }}
            />
            {changeRole.error !== null && (
                <Toast variant="error" onDismiss={changeRole.reset}>
                    {facilitationRefusal(changeRole.error)}
                </Toast>
            )}
            {changeRole.isSuccess && changed !== undefined && (
                <Toast variant="success" onDismiss={changeRole.reset}>
                    <bdi>{changedName}</bdi> {ROLE_CHANGED[changed.role]}
                </Toast>
            )}
            {removal.isSuccess && (
                <Toast variant="success" onDismiss={removal.reset}>
                    <bdi>{removedName}</bdi> was removed from the board.
                </Toast>
            )}
        </>
    );
}
