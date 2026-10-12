import { useState } from "react";
import { useNavigate } from "react-router";
import { Button } from "../../components/Button/Button";
import { ConfirmDialog } from "../../components/ConfirmDialog/ConfirmDialog";
import { FormField } from "../../components/FormField/FormField";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { Panel } from "../../components/Panel/Panel";
import { Select } from "../../components/Select/Select";
import { Toast } from "../../components/Toast/Toast";
import { boardManagementActions } from "../../domain/board/capabilities";
import { boardHome, moveTargets, ORGANISATION_TEAM } from "../../domain/board/organisationBoards";
import { organisationHomePath } from "../../domain/organisations";
import {
    useArchiveBoard,
    useDeleteBoard,
    useMoveBoard,
    useUnarchiveBoard,
} from "../../hooks/mutations/boardManagement";
import { useTeams } from "../../hooks/queries/teams";
import type { BoardSnapshot, OrganisationResponse } from "../../types/api";
import { ActionFailure } from "../org/shared/ActionFailure";
import sectionStyles from "../org/shared/OrgSection.module.css";

interface BoardManagementSectionProps {
    organisation: OrganisationResponse;
    snapshot: BoardSnapshot;
}

export function BoardManagementSection({ organisation, snapshot }: BoardManagementSectionProps) {
    const boardId = snapshot.board_id;
    const actions = boardManagementActions(snapshot);
    const teams = useTeams(organisation.id);
    const move = useMoveBoard(boardId);
    const archive = useArchiveBoard(boardId);
    const unarchive = useUnarchiveBoard(boardId);
    const remove = useDeleteBoard(boardId, organisation.id);
    const navigate = useNavigate();
    const current = boardHome(snapshot.board);
    const [target, setTarget] = useState(current);
    const [moved, setMoved] = useState(false);
    const [confirming, setConfirming] = useState(false);

    const archived = snapshot.board.archived_at !== undefined;
    const targets = moveTargets(teams.data?.teams ?? [], organisation.role, current);
    const failure = move.error ?? archive.error ?? unarchive.error ?? remove.error;

    return (
        <Panel
            title="Manage board"
            headingLevel={2}
            description="Move the board to another team, archive it to make it read-only, or delete it."
        >
            <div className={sectionStyles.stack}>
                {teams.error !== null && <LoadFailure message={teams.error.message} onRetry={teams.refetch} />}
                {failure !== null && <ActionFailure error={failure} />}
                {moved && (
                    <Toast variant="success" onDismiss={() => setMoved(false)}>
                        Board moved.
                    </Toast>
                )}

                {actions.move && (
                    <div className={sectionStyles.inline}>
                        <FormField
                            label="Team"
                            hint="Who can open the board follows its team. Its action items move with it."
                            layout="row"
                            flush
                        >
                            {control => (
                                <Select {...control} value={target} onChange={e => setTarget(e.target.value)}>
                                    {targets.map(option => (
                                        <option key={option.value} value={option.value}>
                                            {option.label}
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </FormField>
                        <Button
                            type="button"
                            disabled={target === current || move.isPending}
                            onClick={() => {
                                setMoved(false);
                                move.mutate(
                                    { teamId: target === ORGANISATION_TEAM ? null : target },
                                    { onSuccess: () => setMoved(true) },
                                );
                            }}
                        >
                            Move board
                        </Button>
                    </div>
                )}

                <div className={sectionStyles.actions}>
                    {actions.archive && !archived && (
                        <Button type="button" disabled={archive.isPending} onClick={() => archive.mutate()}>
                            Archive board
                        </Button>
                    )}
                    {actions.archive && archived && (
                        <Button type="button" disabled={unarchive.isPending} onClick={() => unarchive.mutate()}>
                            Unarchive board
                        </Button>
                    )}
                    {actions.remove && (
                        <Button type="button" variant="danger" onClick={() => setConfirming(true)}>
                            Delete board
                        </Button>
                    )}
                </div>
            </div>

            <ConfirmDialog
                open={confirming}
                title="Delete this board?"
                body={
                    <p>
                        Everyone loses access to <bdi>{snapshot.board.title}</bdi> straight away. You can restore it
                        from Recently deleted on the boards page for 30 days. After that it is gone for good.
                    </p>
                }
                confirmLabel="Delete board"
                destructive
                busy={remove.isPending}
                onConfirm={() =>
                    remove.mutate(undefined, {
                        onSuccess: () => {
                            navigate(organisationHomePath(organisation.slug), { replace: true });
                        },
                        onError: () => setConfirming(false),
                    })
                }
                onCancel={() => setConfirming(false)}
            />
        </Panel>
    );
}
