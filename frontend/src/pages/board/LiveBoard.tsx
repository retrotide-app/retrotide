import { type ReactNode, useCallback, useContext, useEffect, useState } from "react";
import { ActionItemsPanel } from "../../components/board/actionItems/ActionItemsPanel";
import { PreviousActionsPanel } from "../../components/board/actionItems/PreviousActionsPanel";
import { BoardExportMenu } from "../../components/board/BoardExportMenu";
import { BoardViews } from "../../components/board/BoardViews";
import { FacilitationBar } from "../../components/board/FacilitationBar";
import { PeoplePanel } from "../../components/board/PeoplePanel";
import { PresentationView } from "../../components/board/presentation/PresentationView";
import { SpotlightControls } from "../../components/board/presentation/SpotlightControls";
import { ShortcutsDialog } from "../../components/board/ShortcutsDialog";
import { StageBar } from "../../components/board/StageBar";
import { SurveysPanel } from "../../components/board/surveys/SurveysPanel";
import { Banner, BannerDismiss } from "../../components/Banner/Banner";
import { Button } from "../../components/Button/Button";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { StatusTag } from "../../components/StatusTag/StatusTag";
import { Toast } from "../../components/Toast/Toast";
import { CardDeleteUndoProvider } from "../../context/CardDeleteUndoContext";
import { boardActions, canExportBoard, isReadOnly } from "../../domain/board/capabilities";
import { canShareBoard, sharedBoardRefusal } from "../../domain/board/shareLink";
import { refusalNotice } from "../../domain/board/syncStatus";
import { refusedWith } from "../../domain/refusals";
import { useBoardPresence } from "../../hooks/board/useBoardPresence";
import { useBoardShortcuts } from "../../hooks/board/useBoardShortcuts";
import { useBoardView } from "../../hooks/board/useBoardView";
import { type CardFocus, useCardFocus } from "../../hooks/board/useCardFocus";
import { CardDeleteUndoContext } from "../../hooks/board/useDeferredCardDelete";
import { useJoinBoard } from "../../hooks/mutations/boards";
import type { BoardSnapshot, BoardView } from "../../types/api";
import { OrgPage } from "../org/shared/OrgPage";
import { BoardDescription } from "./BoardDescription";
import styles from "./BoardPage.module.css";
import { PrimeDirective } from "./PrimeDirective";
import { ShareDialog } from "./ShareDialog";
import { SharedBoardRefusal } from "./SharedBoardRefusal";

export interface SharedBoardLink {
    rejoinPath: string | null;
}

interface LiveBoardProps {
    boardId: string;
    backLink?: (board: BoardView) => ReactNode;
    settingsPath?: string;
    summaryPath: string;
    link?: SharedBoardLink;
}

interface LoadRefusalProps {
    error: Error;
    link?: SharedBoardLink;
    onRetry: () => unknown;
}

interface BoardOrPresentationProps {
    snapshot: BoardSnapshot;
    writable: boolean;
    cardFocus: CardFocus;
}

const PAGE_TITLE = "Board";

function BoardOrPresentation({ snapshot, writable, cardFocus }: BoardOrPresentationProps) {
    const [left, setLeft] = useState(false);
    const presenting = snapshot.spotlight.presenting_card_id !== null;

    if (presenting && !left) {
        return (
            <PresentationView
                snapshot={snapshot}
                writable={writable}
                cardFocus={cardFocus}
                onLeave={() => setLeft(true)}
            />
        );
    }

    return (
        <>
            {presenting && (
                <Banner
                    tone="info"
                    role="status"
                    label="Presentation"
                    actions={
                        <Button type="button" size="small" onClick={() => setLeft(false)}>
                            Follow
                        </Button>
                    }
                >
                    The facilitator is presenting
                </Banner>
            )}
            <BoardViews snapshot={snapshot} writable={writable} cardFocus={cardFocus} />
        </>
    );
}

function DeletedCardNotice() {
    const deferred = useContext(CardDeleteUndoContext);

    if (deferred === null) {
        return null;
    }

    const latest = deferred.waiting.at(-1);

    return (
        <>
            {deferred.error !== null && (
                <Banner tone="danger" role="alert" actions={<BannerDismiss onDismiss={deferred.dismissError} />}>
                    {`The card could not be deleted: ${deferred.error.message}`}
                </Banner>
            )}
            {latest !== undefined && (
                <Toast duration={0}>
                    <span className={styles.undoNotice}>
                        Card deleted.
                        <Button type="button" size="small" onClick={() => deferred.undo(latest)}>
                            Undo
                        </Button>
                    </span>
                </Toast>
            )}
        </>
    );
}

function LoadRefusal({ error, link, onRetry }: LoadRefusalProps) {
    const refusal = link === undefined ? null : sharedBoardRefusal(error, link.rejoinPath !== null);

    if (link !== undefined && refusal !== null) {
        return <SharedBoardRefusal message={refusal} rejoinPath={link.rejoinPath} />;
    }

    return <LoadFailure message={error.message} onRetry={onRetry} />;
}

export function LiveBoard({ boardId, backLink, settingsPath, summaryPath, link }: LiveBoardProps) {
    const board = useBoardView(boardId);
    const online = useBoardPresence(boardId);
    const join = useJoinBoard(boardId);
    const cardFocus = useCardFocus();
    const { mutate: joinBoard } = join;
    const [sharing, setSharing] = useState(false);
    const [showingShortcuts, setShowingShortcuts] = useState(false);
    const showShortcuts = useCallback(() => setShowingShortcuts(true), []);

    useEffect(() => {
        joinBoard();
    }, [joinBoard]);

    const presenting = (board.data?.spotlight.presenting_card_id ?? null) !== null;

    useBoardShortcuts({ active: board.data !== undefined && !presenting, onHelp: showShortcuts });

    const highlighted = board.data?.spotlight.highlighted_card_id ?? null;

    useEffect(() => {
        if (highlighted !== null) {
            cardFocus.reveal(highlighted);
        }
    }, [cardFocus, highlighted]);

    const snapshot = board.data;
    const failure = board.error !== null && <LoadRefusal error={board.error} link={link} onRetry={board.refetch} />;

    if (snapshot === undefined) {
        return (
            <OrgPage title={PAGE_TITLE}>
                {failure}
                {board.error === null && <p className="loading">Loading board...</p>}
            </OrgPage>
        );
    }

    const { board: details } = snapshot;
    const readOnly = isReadOnly(details);
    const ended = details.closed;
    const joined = join.isSuccess;
    const joinError = refusedWith(join.error, ["archived"]) ? null : join.error;
    const actions = boardActions(snapshot);
    const configures = joined && !readOnly && (actions.manageColumns || actions.describeBoard);
    const shares = joined && canShareBoard(snapshot);
    const { sync } = board;
    const syncNotice = refusalNotice(sync);
    const back = backLink?.(details);
    const backdrop = details.background_path;

    const badge = (
        <>
            {ended && <StatusTag tone="neutral">Ended</StatusTag>}
            {readOnly && !ended && <StatusTag tone="neutral">Archived</StatusTag>}
            {!readOnly && sync.state === "live" && <StatusTag tone="success">Live</StatusTag>}
            {!readOnly && sync.state === "connecting" && <StatusTag tone="warning">Connecting</StatusTag>}
        </>
    );

    const status = (
        <>
            {shares && (
                <Button type="button" variant="primary" onClick={() => setSharing(true)}>
                    Share
                </Button>
            )}
            <ButtonLink to={summaryPath}>Summary</ButtonLink>
            {canExportBoard(snapshot) && <BoardExportMenu boardId={boardId} snapshot={snapshot} />}
            {configures && settingsPath !== undefined && <ButtonLink to={settingsPath}>Board settings</ButtonLink>}
            <Button type="button" variant="ghost" onClick={showShortcuts}>
                Keyboard shortcuts
            </Button>
        </>
    );

    return (
        <OrgPage title={details.title} badge={badge} status={status}>
            {(back !== undefined || details.creator_name !== undefined) && (
                <p>
                    {back}
                    {details.creator_name !== undefined && (
                        <span className={styles.muted}>
                            {back === undefined ? "" : " · "}Created by <bdi>{details.creator_name}</bdi>
                        </span>
                    )}
                </p>
            )}
            <div className={styles.stack} data-layout="full-width">
                {failure}
                {syncNotice !== null && link === undefined && <ErrorBanner message={syncNotice} />}
                {syncNotice !== null && link !== undefined && (
                    <SharedBoardRefusal message={syncNotice} rejoinPath={link.rejoinPath} />
                )}
                {joinError !== null && <LoadFailure message={joinError.message} onRetry={join.mutate} />}
                <StageBar board={details} />
                {readOnly && !ended && <p>This board is archived, so it can only be read.</p>}
                <BoardDescription board={details} editable={false} />
                {details.show_prime_directive && <PrimeDirective />}
                {details.cards_hidden && (
                    <p className={styles.muted}>
                        Cards are hidden. You see your own cards in full, and everyone else&apos;s as blurred cards.
                    </p>
                )}
                {details.anonymity === "anonymous" && !ended && (
                    <p className={styles.muted}>
                        This board is anonymous. Nobody, facilitators included, can see who wrote a card.
                    </p>
                )}
                {details.anonymity === "anonymous" && ended && (
                    <p className={styles.muted}>
                        This board was anonymous, and the link between people and their cards was deleted when the retro
                        ended.
                    </p>
                )}
                <p className="visually-hidden" role="status">
                    {details.cards_hidden ? "Cards hidden." : "Cards revealed."}
                </p>
                <FacilitationBar snapshot={snapshot} writable={joined} />
                <PreviousActionsPanel snapshot={snapshot} writable={joined} />
                <ActionItemsPanel snapshot={snapshot} writable={joined} cardFocus={cardFocus} />
                <SurveysPanel snapshot={snapshot} writable={joined} />
                <SpotlightControls snapshot={snapshot} writable={joined} />
                <CardDeleteUndoProvider boardId={boardId}>
                    <DeletedCardNotice />
                    <div
                        className={backdrop === undefined ? styles.canvas : `${styles.canvas} ${styles.backdrop}`}
                        style={backdrop === undefined ? undefined : { backgroundImage: `url("${backdrop}")` }}
                    >
                        <BoardOrPresentation
                            key={presenting ? "presentation" : "board"}
                            snapshot={snapshot}
                            writable={joined}
                            cardFocus={cardFocus}
                        />
                    </div>
                </CardDeleteUndoProvider>
                <PeoplePanel snapshot={snapshot} online={online} />
            </div>
            {sharing && <ShareDialog boardId={boardId} onClose={() => setSharing(false)} />}
            <ShortcutsDialog open={showingShortcuts} onClose={() => setShowingShortcuts(false)} />
        </OrgPage>
    );
}
