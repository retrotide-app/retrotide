import { type ReactNode, useState } from "react";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import styles from "./ReasonDialog.module.css";

interface ReasonDialogProps {
    title: string;
    body: ReactNode;
    confirmLabel: string;
    busy: boolean;
    error: Error | null;
    onConfirm: (reason: string) => void;
    onCancel: () => void;
}

const REASON_MISSING = "Give a reason.";

export function ReasonDialog({ title, body, confirmLabel, busy, error, onConfirm, onCancel }: ReasonDialogProps) {
    const [reason, setReason] = useState("");
    const [reasonError, setReasonError] = useState<string | null>(null);

    const summary = reasonError ?? error?.message ?? null;

    function handleConfirm() {
        const trimmed = reason.trim();

        if (trimmed === "") {
            setReasonError(REASON_MISSING);
            return;
        }

        setReasonError(null);
        onConfirm(trimmed);
    }

    const content = (
        <div className={styles.content}>
            <p>{body}</p>
            {summary !== null && <ErrorBanner message={summary} takeFocus />}
            <TextField
                label="Reason"
                value={reason}
                onChange={e => setReason(e.target.value)}
                error={reasonError}
                disabled={busy}
            />
        </div>
    );

    return (
        <ConfirmDialog
            open
            title={title}
            body={content}
            confirmLabel={confirmLabel}
            destructive
            busy={busy}
            onConfirm={handleConfirm}
            onCancel={onCancel}
        />
    );
}
