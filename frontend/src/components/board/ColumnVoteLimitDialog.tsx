import { useState } from "react";
import { facilitationRefusal } from "../../domain/board/facilitation";
import { COLUMN_VOTE_LIMIT_MAX, columnVoteLimitFrom } from "../../domain/board/votes";
import { useSetColumnVoteLimit } from "../../hooks/mutations/facilitation";
import type { ColumnView } from "../../types/api";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { FormDialog } from "../FormDialog/FormDialog";
import { TextField } from "../FormField/TextField";
import { ToggleSwitch } from "../ToggleSwitch/ToggleSwitch";

interface ColumnVoteLimitDialogProps {
    boardId: string;
    column: ColumnView;
    onClose: () => void;
}

const LIMIT_PROBLEM = `Enter a whole number from 1 to ${COLUMN_VOTE_LIMIT_MAX}.`;

export function ColumnVoteLimitDialog({ boardId, column, onClose }: ColumnVoteLimitDialogProps) {
    const limit = useSetColumnVoteLimit(boardId);
    const [unlimited, setUnlimited] = useState(column.max_votes === null);
    const [text, setText] = useState(column.max_votes === null ? "" : String(column.max_votes));
    const [problem, setProblem] = useState<string | null>(null);

    const refusal = limit.error === null ? null : facilitationRefusal(limit.error);
    const failure = problem ?? refusal;

    function handleSubmit() {
        const maxVotes = unlimited ? null : columnVoteLimitFrom(text);

        if (!unlimited && maxVotes === null) {
            setProblem(LIMIT_PROBLEM);
            return;
        }

        setProblem(null);
        limit.mutate({ columnId: column.id, maxVotes }, { onSuccess: onClose });
    }

    return (
        <FormDialog
            open
            title={`Vote limit for ${column.title}`}
            submitLabel="Save vote limit"
            busy={limit.isPending}
            onSubmit={handleSubmit}
            onCancel={onClose}
        >
            {failure !== null && <ErrorBanner message={failure} takeFocus />}
            <ToggleSwitch
                label="No column limit"
                description="Only the board's own vote limits apply to cards in this column."
                enabled={unlimited}
                disabled={limit.isPending}
                onChange={setUnlimited}
            />
            <TextField
                label="Votes per person in this column"
                hint={`A whole number from 1 to ${COLUMN_VOTE_LIMIT_MAX}. Votes already cast are kept.`}
                type="number"
                inputMode="numeric"
                min={1}
                max={COLUMN_VOTE_LIMIT_MAX}
                value={text}
                disabled={unlimited || limit.isPending}
                onChange={e => setText(e.target.value)}
            />
        </FormDialog>
    );
}
