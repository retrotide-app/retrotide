import { titleFromCard } from "../../../domain/board/actionItems";
import { actionItemActions } from "../../../domain/board/capabilities";
import { useBoardMembership } from "../../../hooks/board/useBoardMembership";
import { useAddActionItem } from "../../../hooks/mutations/actionItems";
import type { BoardSnapshot, CardView } from "../../../types/api";
import { LoadFailure } from "../../LoadFailure/LoadFailure";
import { Modal } from "../../Modal/Modal";
import { ActionItemForm, type ActionItemSubmission } from "./ActionItemForm";

interface MakeActionItemDialogProps {
    snapshot: BoardSnapshot;
    card: CardView;
    onClose: () => void;
}

export function MakeActionItemDialog({ snapshot, card, onClose }: MakeActionItemDialogProps) {
    const membership = useBoardMembership(snapshot.board_id, true);
    const add = useAddActionItem(snapshot.board_id);

    const actions = actionItemActions(snapshot, membership.member);

    function handleSubmit(fields: ActionItemSubmission) {
        add.mutate({ ...fields, source_card_id: card.id }, { onSuccess: onClose });
    }

    return (
        <Modal isOpen onClose={onClose} title="Make action item">
            {membership.failure !== null && (
                <LoadFailure message={membership.failure.message} onRetry={membership.retry} />
            )}
            <ActionItemForm
                label="Make action item"
                initial={{
                    title: titleFromCard(card.body),
                    assignee: null,
                    dueOn: null,
                    mentions: [],
                }}
                people={actions.name ? membership.people : null}
                submitLabel="Add action item"
                pending={add.isPending}
                error={add.error}
                autoFocus
                onSubmit={handleSubmit}
                onCancel={onClose}
            />
        </Modal>
    );
}
