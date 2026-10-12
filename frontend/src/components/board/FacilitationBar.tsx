import { useState } from "react";
import { boardActions, boardInteractions, isReadOnly } from "../../domain/board/capabilities";
import { activeLocks, facilitationRefusal } from "../../domain/board/facilitation";
import { backStep, nextStep, STAGE_INFO, type StageStep } from "../../domain/board/stages";
import { useChangeStage, useCloseBoard } from "../../hooks/mutations/boards";
import type { BoardSnapshot, BoardStage } from "../../types/api";
import { Banner } from "../Banner/Banner";
import { Button } from "../Button/Button";
import { ConfirmDialog } from "../ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { StatusTag } from "../StatusTag/StatusTag";
import { Toast } from "../Toast/Toast";
import { BoardTimer } from "./BoardTimer";
import styles from "./FacilitationBar.module.css";

interface FacilitationBarProps {
    snapshot: BoardSnapshot;
    writable: boolean;
}

interface Confirming {
    kind: "reveal" | "end";
    stage: BoardStage | null;
}

export function FacilitationBar({ snapshot, writable }: FacilitationBarProps) {
    const advance = useChangeStage(snapshot.board_id);
    const close = useCloseBoard(snapshot.board_id);
    const [confirming, setConfirming] = useState<Confirming | null>(null);

    const { board, settings, timer } = snapshot;
    const facilitates = writable && boardInteractions(snapshot).facilitate;
    const controlsTimer = writable && boardActions(snapshot).controlTimer;
    const locks = isReadOnly(board) ? [] : activeLocks(settings);
    const anonymous = board.anonymity === "anonymous";
    const step = nextStep(board);
    const back = backStep(board);
    const pending = confirming?.stage === null ? close : advance;

    if (!facilitates && !controlsTimer && timer === null && locks.length === 0) {
        return null;
    }

    function openConfirm(next: Confirming) {
        advance.reset();
        close.reset();
        setConfirming(next);
    }

    function handleNextStage(target: StageStep) {
        if (target.revealsCards) {
            openConfirm({ kind: "reveal", stage: target.to });
            return;
        }

        if (target.endsRetro) {
            openConfirm({ kind: "end", stage: target.to });
            return;
        }

        close.reset();
        advance.reset();
        advance.mutate({ stage: target.to });
    }

    function handleBack(stage: BoardStage) {
        close.reset();
        advance.reset();
        advance.mutate({ stage });
    }

    function handleConfirm() {
        if (confirming === null) {
            return;
        }

        const settle = {
            onSuccess: () => {
                setConfirming(null);
            },
        };

        if (confirming.stage === null) {
            close.mutate(undefined, settle);
            return;
        }

        advance.mutate({ stage: confirming.stage }, settle);
    }

    function handleCancel() {
        advance.reset();
        close.reset();
        setConfirming(null);
    }

    const confirmBody = (
        <div className={styles.dialog}>
            {confirming?.kind === "reveal" && (
                <>
                    <p>
                        Everyone&apos;s cards are shown to the whole board at once, and voting starts. Cards cannot be
                        hidden again after this.
                    </p>
                    {anonymous && (
                        <p>This board is anonymous, so the cards in each column are shuffled as they are revealed.</p>
                    )}
                </>
            )}
            {confirming?.kind === "end" && (
                <>
                    <p>
                        The board becomes read-only for everyone and the timer stops. Any card that is still hidden is
                        shown.
                    </p>
                    {anonymous && (
                        <p>
                            This board is anonymous, so the link between people and their cards and votes is deleted for
                            good. Vote totals stay.
                        </p>
                    )}
                    <p>This cannot be undone.</p>
                </>
            )}
            {pending.error !== null && <ErrorBanner message={facilitationRefusal(pending.error)} takeFocus />}
        </div>
    );

    return (
        <div className={styles.bar} role="region" aria-label="Facilitation">
            <BoardTimer snapshot={snapshot} controls={controlsTimer} facilitates={facilitates} />
            {facilitates && locks.length > 0 && (
                <div className={styles.locks}>
                    {locks.map(lock => (
                        <StatusTag key={lock.name} tone="warning">
                            {lock.label}
                        </StatusTag>
                    ))}
                </div>
            )}
            {facilitates && (
                <div className={styles.actions}>
                    {back !== null && (
                        <Button
                            type="button"
                            size="small"
                            disabled={advance.isPending}
                            onClick={() => handleBack(back)}
                        >
                            {`Back to ${STAGE_INFO[back].label}`}
                        </Button>
                    )}
                    {step !== null && (
                        <Button
                            type="button"
                            size="small"
                            variant="primary"
                            disabled={advance.isPending}
                            onClick={() => handleNextStage(step)}
                        >
                            {`Next stage: ${step.label}`}
                        </Button>
                    )}
                    {step?.endsRetro !== true && (
                        <Button
                            type="button"
                            size="small"
                            variant="danger"
                            onClick={() => openConfirm({ kind: "end", stage: null })}
                        >
                            End retro
                        </Button>
                    )}
                </div>
            )}
            {!facilitates && locks.length > 0 && (
                <div className={styles.notice}>
                    <Banner tone="warning" role="status" label="Locks">
                        {locks.map(lock => lock.notice).join(" ")}
                    </Banner>
                </div>
            )}
            <ConfirmDialog
                open={confirming !== null}
                title={confirming?.kind === "reveal" ? "Reveal the cards and start voting?" : "End this retro?"}
                body={confirmBody}
                confirmLabel={confirming?.kind === "reveal" ? "Reveal cards" : "End retro"}
                destructive={confirming?.kind === "end"}
                busy={pending.isPending}
                onConfirm={handleConfirm}
                onCancel={handleCancel}
            />
            {confirming === null && advance.error !== null && (
                <Toast variant="error" onDismiss={advance.reset}>
                    {facilitationRefusal(advance.error)}
                </Toast>
            )}
        </div>
    );
}
