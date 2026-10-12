import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import { StatusTag } from "../../components/StatusTag/StatusTag";
import { TextLink } from "../../components/TextLink/TextLink";
import { ACTION_ITEM_STATUS_LABELS, formatDueDate } from "../../domain/board/actionItems";
import { MENTION_KIND_LABELS, mentionKey, mentionText } from "../../domain/board/mentions";
import { boardPath } from "../../domain/board/paths";
import { useMentions } from "../../hooks/queries/actionItems";
import type { MentionView } from "../../types/api";
import styles from "./AccountSection.module.css";

interface MentionRowProps {
    mention: MentionView;
}

interface MentionBoardProps {
    orgSlug: string;
    boardId: string | undefined;
    boardTitle: string | undefined;
}

function MentionBoard({ orgSlug, boardId, boardTitle }: MentionBoardProps) {
    if (boardId === undefined) {
        return "Board deleted";
    }

    return (
        <TextLink to={boardPath(orgSlug, boardId)}>
            <bdi>{boardTitle ?? "Open the board"}</bdi>
        </TextLink>
    );
}

function MentionRow({ mention }: MentionRowProps) {
    const due = mention.kind === "action_item" ? formatDueDate(mention.due_on) : "";

    return (
        <li className={styles.item}>
            <span className={styles.itemText}>
                <span className={styles.itemName} dir="auto">
                    {mentionText(mention)}
                </span>
                <span className={styles.itemMeta}>
                    {MENTION_KIND_LABELS[mention.kind]}
                    {" · "}
                    <bdi>{mention.org_name}</bdi>
                    {" · "}
                    <MentionBoard
                        orgSlug={mention.org_slug}
                        boardId={mention.board_id}
                        boardTitle={mention.board_title}
                    />
                    {due !== "" && ` · Due ${due}`}
                </span>
            </span>
            {mention.kind === "action_item" && (
                <StatusTag tone={mention.status === "done" ? "success" : "neutral"}>
                    {ACTION_ITEM_STATUS_LABELS[mention.status]}
                </StatusTag>
            )}
        </li>
    );
}

export function MentionsSection() {
    const mentions = useMentions();
    const listed = mentions.data?.mentions;

    return (
        <SettingsRow
            title="Mentions"
            headingLevel={2}
            description="Action items, cards and comments you were mentioned in, newest first, on the boards you can still open."
        >
            {mentions.error !== null && <LoadFailure message={mentions.error.message} onRetry={mentions.refetch} />}
            {listed === undefined && mentions.error === null && <p className="loading">Loading mentions...</p>}
            {listed !== undefined && listed.length === 0 && (
                <p className={styles.text}>Nobody has mentioned you yet.</p>
            )}
            {listed !== undefined && listed.length > 0 && (
                <ul className={styles.list}>
                    {listed.map(mention => (
                        <MentionRow key={mentionKey(mention)} mention={mention} />
                    ))}
                </ul>
            )}
        </SettingsRow>
    );
}
