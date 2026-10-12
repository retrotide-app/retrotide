import { type SubmitEvent, useState } from "react";
import { Button } from "../../components/Button/Button";
import { ConfirmDialog } from "../../components/ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../components/FormField/TextField";
import { Panel } from "../../components/Panel/Panel";
import { ToggleGroup, type ToggleOption } from "../../components/ToggleGroup/ToggleGroup";
import { ToggleSwitch } from "../../components/ToggleSwitch/ToggleSwitch";
import { VOTE_LIMIT_MAX, voteLimitFrom } from "../../domain/board/fields";
import {
    type BoardSettingsChange,
    useHideCards,
    useResetVotes,
    useRevealCards,
    useUpdateBoardSettings,
} from "../../hooks/mutations/boards";
import type { BoardSettingsView, BoardSnapshot, SortMode } from "../../types/api";
import styles from "./BoardSettingsPage.module.css";

interface BoardInteractionSettingsProps {
    snapshot: BoardSnapshot;
}

interface VoteLimitsProps {
    settings: BoardSettingsView;
    pending: boolean;
    onSave: (change: BoardSettingsChange) => void;
}

const SORT_OPTIONS: readonly ToggleOption<SortMode>[] = [
    { value: "manual", label: "Board order" },
    { value: "votes", label: "Most votes" },
];

const LIMIT_PROBLEM = `Enter a whole number from 0 to ${VOTE_LIMIT_MAX}.`;

function VoteLimits({ settings, pending, onSave }: VoteLimitsProps) {
    const [perPerson, setPerPerson] = useState(String(settings.max_votes));
    const [perCard, setPerCard] = useState(String(settings.max_votes_per_card));
    const [problem, setProblem] = useState<string | null>(null);

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        const maxVotes = voteLimitFrom(perPerson);
        const maxVotesPerCard = voteLimitFrom(perCard);

        if (maxVotes === null || maxVotesPerCard === null) {
            setProblem(LIMIT_PROBLEM);
            return;
        }

        setProblem(null);
        onSave({ max_votes: maxVotes, max_votes_per_card: maxVotesPerCard });
    }

    return (
        <form className={styles.limits} onSubmit={handleSubmit} aria-label="Vote limits" noValidate>
            {problem !== null && <ErrorBanner message={problem} takeFocus />}
            <TextField
                label="Votes per person"
                flush
                type="number"
                inputMode="numeric"
                min={0}
                max={VOTE_LIMIT_MAX}
                value={perPerson}
                onChange={e => setPerPerson(e.target.value)}
            />
            <TextField
                label="Votes per card"
                flush
                type="number"
                inputMode="numeric"
                min={0}
                max={VOTE_LIMIT_MAX}
                value={perCard}
                onChange={e => setPerCard(e.target.value)}
            />
            <Button type="submit" disabled={pending}>
                Save limits
            </Button>
            <p className={styles.limitsNote}>Votes per card at 0 lets anyone put all their votes on one card.</p>
        </form>
    );
}

export function BoardInteractionSettings({ snapshot }: BoardInteractionSettingsProps) {
    const update = useUpdateBoardSettings(snapshot.board_id);
    const reset = useResetVotes(snapshot.board_id);
    const reveal = useRevealCards(snapshot.board_id);
    const hide = useHideCards(snapshot.board_id);
    const [confirmingReset, setConfirmingReset] = useState(false);
    const { board, settings } = snapshot;
    const { mutate: change } = update;
    const visibilityError = reveal.error ?? hide.error;

    return (
        <>
            <Panel
                title="Voting"
                headingLevel={2}
                description="How everyone votes on cards. Changes reach the board straight away."
            >
                {update.error !== null && <ErrorBanner message={update.error.message} takeFocus />}
                <div className={styles.toggles}>
                    <ToggleSwitch
                        label="Voting open"
                        description="Let participants vote on cards and take votes back in the Vote stage."
                        enabled={settings.voting_open}
                        disabled={update.isPending}
                        onChange={next => change({ voting_open: next })}
                    />
                    <ToggleSwitch
                        label="Hide vote totals"
                        description="Everyone sees only their own votes until the board moves on to Discuss."
                        enabled={settings.vote_counts_hidden}
                        disabled={update.isPending}
                        onChange={next => change({ vote_counts_hidden: next })}
                    />
                </div>
                <VoteLimits
                    key={`${settings.max_votes}/${settings.max_votes_per_card}`}
                    settings={settings}
                    pending={update.isPending}
                    onSave={change}
                />
                <div className={styles.inline}>
                    <span className={styles.inlineLabel}>Card order everyone starts with</span>
                    <ToggleGroup
                        label="Card order everyone starts with"
                        options={SORT_OPTIONS}
                        value={settings.sort_mode}
                        onChange={sortMode => change({ sort_mode: sortMode })}
                    />
                </div>
                <div className={styles.toggles}>
                    <ToggleSwitch
                        label="Enforce sort order"
                        description="Everyone sees the board's sort order"
                        enabled={settings.sort_enforced}
                        disabled={update.isPending}
                        onChange={next => change({ sort_enforced: next })}
                    />
                </div>
                <div className={styles.inline}>
                    <Button
                        type="button"
                        size="small"
                        variant="danger"
                        onClick={() => {
                            reset.reset();
                            setConfirmingReset(true);
                        }}
                    >
                        Reset votes
                    </Button>
                </div>
            </Panel>
            <Panel title="Cards" headingLevel={2} description="What people can do with each other's cards.">
                {visibilityError !== null && <ErrorBanner message={visibilityError.message} takeFocus />}
                <div className={styles.toggles}>
                    {board.stage === "write" && (
                        <ToggleSwitch
                            label="Cards hidden"
                            description={
                                board.cards_hidden
                                    ? "Everyone sees only their own cards until voting starts."
                                    : "Hiding again only blurs the cards. Anyone who has read a card has still read it."
                            }
                            enabled={board.cards_hidden}
                            disabled={reveal.isPending || hide.isPending}
                            onChange={next => {
                                reveal.reset();
                                hide.reset();

                                if (next) {
                                    hide.mutate();
                                    return;
                                }

                                reveal.mutate();
                            }}
                        />
                    )}
                    <ToggleSwitch
                        label="Lock cards"
                        description="Participants cannot add, edit or delete cards. Facilitators still can."
                        enabled={settings.cards_locked}
                        disabled={update.isPending}
                        onChange={next => change({ cards_locked: next })}
                    />
                    <ToggleSwitch
                        label="Lock moving"
                        description="Participants cannot move or merge cards. Facilitators still can."
                        enabled={settings.moves_locked}
                        disabled={update.isPending}
                        onChange={next => change({ moves_locked: next })}
                    />
                    <ToggleSwitch
                        label="Comments"
                        description="Let participants comment on cards in the Discuss stage."
                        enabled={settings.comments_enabled}
                        disabled={update.isPending}
                        onChange={next => change({ comments_enabled: next })}
                    />
                    <ToggleSwitch
                        label="Merging"
                        description="Let participants drop a card on another to merge them in the Discuss stage."
                        enabled={settings.merging_enabled}
                        disabled={update.isPending}
                        onChange={next => change({ merging_enabled: next })}
                    />
                    <ToggleSwitch
                        label="Show card dates"
                        description={
                            board.anonymity === "anonymous"
                                ? "This board is anonymous, so cards never show when they were written, even with this on."
                                : "Show on each card the date it was written."
                        }
                        enabled={settings.show_card_dates}
                        disabled={update.isPending}
                        onChange={next => change({ show_card_dates: next })}
                    />
                </div>
            </Panel>
            <Panel title="Board" headingLevel={2} description="The whole board and its timer.">
                <div className={styles.toggles}>
                    <ToggleSwitch
                        label="Lock the board"
                        description="Everyone can read the board, but only facilitators can change anything."
                        enabled={settings.board_locked}
                        disabled={update.isPending}
                        onChange={next => change({ board_locked: next })}
                    />
                    <ToggleSwitch
                        label="Timer sound"
                        description="Play a short chime for everyone when the timer runs out."
                        enabled={settings.timer_play_sound}
                        disabled={update.isPending}
                        onChange={next => change({ timer_play_sound: next })}
                    />
                    <ToggleSwitch
                        label="Lock the board when the timer ends"
                        description="When a running timer reaches zero, the board locks as if a facilitator locked it."
                        enabled={settings.timer_auto_lock}
                        disabled={update.isPending}
                        onChange={next => change({ timer_auto_lock: next })}
                    />
                    <ToggleSwitch
                        label="Anyone can edit"
                        description="Participants can run the timer and add, edit and move columns"
                        enabled={settings.anyone_can_edit}
                        disabled={update.isPending}
                        onChange={next => change({ anyone_can_edit: next })}
                    />
                </div>
            </Panel>
            <ConfirmDialog
                open={confirmingReset}
                title="Reset every vote?"
                body={
                    <div className={styles.dialog}>
                        <p>Every vote on the board is removed and everyone gets all their votes back.</p>
                        {reset.error !== null && <ErrorBanner message={reset.error.message} takeFocus />}
                    </div>
                }
                confirmLabel="Reset votes"
                destructive
                busy={reset.isPending}
                onConfirm={() => reset.mutate(undefined, { onSuccess: () => setConfirmingReset(false) })}
                onCancel={() => setConfirmingReset(false)}
            />
        </>
    );
}
