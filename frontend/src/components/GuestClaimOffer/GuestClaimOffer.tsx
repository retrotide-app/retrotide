import { useState } from "react";
import { useClaimGuestBoards, useDeclineGuestClaim } from "../../hooks/mutations/guestClaim";
import { useGuestClaimOffer } from "../../hooks/queries/guestClaim";
import { useAuth } from "../../hooks/useAuth";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { FormDialog } from "../FormDialog/FormDialog";
import { Toast } from "../Toast/Toast";
import { ToggleSwitch } from "../ToggleSwitch/ToggleSwitch";
import styles from "./GuestClaimOffer.module.css";

const NO_BOARDS: ReadonlySet<string> = new Set();

export function GuestClaimOffer() {
    const { user } = useAuth();
    const offer = useGuestClaimOffer(user?.id ?? null);
    const claim = useClaimGuestBoards();
    const decline = useDeclineGuestClaim();
    const [leftBehind, setLeftBehind] = useState<ReadonlySet<string>>(NO_BOARDS);
    const [checkFailureSeen, setCheckFailureSeen] = useState(false);

    if (user === null) {
        return null;
    }

    if (claim.isSuccess) {
        return (
            <Toast variant="success" onDismiss={claim.reset}>
                Your guest cards are now in your account.
            </Toast>
        );
    }

    if (offer.error !== null && !checkFailureSeen) {
        return (
            <Toast variant="error" onDismiss={() => setCheckFailureSeen(true)}>
                {`Checking for boards this browser joined as a guest failed: ${offer.error.message}`}
            </Toast>
        );
    }

    const boards = offer.data?.boards ?? [];

    if (boards.length === 0) {
        return null;
    }

    const busy = claim.isPending || decline.isPending;
    const failure = claim.error ?? decline.error;

    function toggle(boardId: string, kept: boolean) {
        setLeftBehind(current => {
            const next = new Set(current);

            if (kept) {
                next.delete(boardId);
            } else {
                next.add(boardId);
            }

            return next;
        });
    }

    function handleClaim() {
        const chosen = boards.filter(board => !leftBehind.has(board.board_id)).map(board => board.board_id);

        if (chosen.length === 0) {
            decline.mutate();
            return;
        }

        claim.mutate(chosen);
    }

    return (
        <FormDialog
            open
            title="Bring your guest cards with you?"
            submitLabel="Bring them over"
            cancelLabel="No thanks"
            busy={busy}
            onSubmit={handleClaim}
            onCancel={() => decline.mutate()}
        >
            <p className={styles.lead}>
                Before you signed in, this browser joined these boards as a guest. Bring your cards on them into your
                account, so they show as yours.
            </p>
            {failure !== null && <ErrorBanner message={failure.message} takeFocus />}
            <div className={styles.boards}>
                {boards.map(board => (
                    <ToggleSwitch
                        key={board.board_id}
                        label={board.title}
                        description={`You joined as ${board.display_name}`}
                        enabled={!leftBehind.has(board.board_id)}
                        onChange={kept => toggle(board.board_id, kept)}
                        disabled={busy}
                    />
                ))}
            </div>
        </FormDialog>
    );
}
