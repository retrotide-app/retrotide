import { type KeyboardEvent, type SubmitEvent, useId, useState } from "react";
import { commentActions, NO_COMMENT_ACTIONS } from "../../domain/board/capabilities";
import { commentCountLabel, commentsOnStack, isEdited } from "../../domain/board/comments";
import { COMMENT_BODY_MAX_LENGTH } from "../../domain/board/fields";
import { keptMentions, NO_MENTIONS } from "../../domain/board/mentions";
import { useCreateComment, useDeleteComment, useEditComment } from "../../hooks/mutations/boards";
import type { BoardSnapshot, CardView, CommentView, PersonView } from "../../types/api";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { Menu, type MenuItem } from "../Menu/Menu";
import styles from "./BoardCard.module.css";
import { MentionText } from "./mentions/MentionText";
import { MentionTextArea } from "./mentions/MentionTextArea";

interface CardCommentsProps {
    snapshot: BoardSnapshot;
    stack: readonly CardView[];
    canComment: boolean;
    writable: boolean;
}

interface CommentBoxProps {
    boardId: string;
    label: string;
    submitLabel: string;
    initialBody: string;
    initialMentions: readonly PersonView[];
    pending: boolean;
    error: Error | null;
    onSubmit: (body: string, mentions: string[], done: () => void) => void;
    onCancel?: () => void;
}

interface CommentItemProps {
    snapshot: BoardSnapshot;
    comment: CommentView;
    writable: boolean;
}

const BODY_MISSING = "Write something in the comment first.";

function CommentBox({
    boardId,
    label,
    submitLabel,
    initialBody,
    initialMentions,
    pending,
    error,
    onSubmit,
    onCancel,
}: CommentBoxProps) {
    const [body, setBody] = useState(initialBody);
    const [mentions, setMentions] = useState<readonly PersonView[]>(initialMentions);
    const [problem, setProblem] = useState<string | null>(null);

    function submit() {
        if (body.trim() === "") {
            setProblem(BODY_MISSING);
            return;
        }

        setProblem(null);
        onSubmit(
            body,
            keptMentions(body, mentions).map(person => person.user_id),
            () => {
                setBody("");
                setMentions([]);
            },
        );
    }

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();
        submit();
    }

    function handleKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
        if (e.key === "Escape" && onCancel !== undefined) {
            onCancel();
            return;
        }

        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            submit();
        }
    }

    const message = problem ?? error?.message ?? null;

    return (
        <form className={styles.commentForm} onSubmit={handleSubmit} aria-label={label} noValidate>
            {message !== null && <ErrorBanner message={message} takeFocus />}
            <MentionTextArea
                boardId={boardId}
                aria-label="Comment text"
                placeholder="Write a comment, then press Enter"
                text={body}
                mentions={mentions}
                onTextChange={(nextBody, nextMentions) => {
                    setBody(nextBody);
                    setMentions(nextMentions);
                }}
                onKeyDown={handleKeyDown}
                maxLength={COMMENT_BODY_MAX_LENGTH}
                rows={2}
                compact
            />
            <div className={styles.commentActions}>
                <Button type="submit" size="small" variant="primary" disabled={pending}>
                    {submitLabel}
                </Button>
                {onCancel !== undefined && (
                    <Button type="button" size="small" variant="ghost" onClick={onCancel}>
                        Cancel
                    </Button>
                )}
            </div>
        </form>
    );
}

function CommentItem({ snapshot, comment, writable }: CommentItemProps) {
    const edit = useEditComment(snapshot.board_id);
    const removal = useDeleteComment(snapshot.board_id);
    const [editing, setEditing] = useState(false);

    const mentions = comment.mentions ?? NO_MENTIONS;
    const actions = writable ? commentActions(snapshot, comment) : NO_COMMENT_ACTIONS;
    const items: MenuItem[] = [];

    if (actions.edit) {
        items.push({
            label: "Edit comment",
            onSelect: () => {
                edit.reset();
                setEditing(true);
            },
        });
    }

    if (actions.remove) {
        items.push({
            label: "Delete comment",
            tone: "danger",
            onSelect: () => removal.mutate({ commentId: comment.id }),
        });
    }

    if (editing) {
        return (
            <li className={styles.comment}>
                <CommentBox
                    boardId={snapshot.board_id}
                    label="Edit comment"
                    submitLabel="Save comment"
                    initialBody={comment.body}
                    initialMentions={mentions}
                    pending={edit.isPending}
                    error={edit.error}
                    onSubmit={(body, named) =>
                        edit.mutate(
                            { commentId: comment.id, body, mentions: named },
                            { onSuccess: () => setEditing(false) },
                        )
                    }
                    onCancel={() => setEditing(false)}
                />
            </li>
        );
    }

    return (
        <li className={styles.comment}>
            {removal.error !== null && <ErrorBanner message={removal.error.message} />}
            <div className={styles.top}>
                <p className={styles.commentBody} dir="auto">
                    <MentionText text={comment.body} mentions={mentions} />
                </p>
                {items.length > 0 && <Menu label="Comment options" items={items} iconOnly size="small" />}
            </div>
            {(comment.author !== null || isEdited(comment)) && (
                <p className={styles.meta}>
                    {comment.author !== null && <bdi>{comment.author.display_name}</bdi>}
                    {comment.author !== null && isEdited(comment) && " · "}
                    {isEdited(comment) && "edited"}
                </p>
            )}
        </li>
    );
}

export function CardComments({ snapshot, stack, canComment, writable }: CardCommentsProps) {
    const threadId = useId();
    const create = useCreateComment(snapshot.board_id);
    const [open, setOpen] = useState(false);

    const [top] = stack;
    const comments = commentsOnStack(snapshot.comments, stack);

    if (comments.length === 0 && !canComment) {
        return null;
    }

    return (
        <>
            <div className={styles.commentToggle}>
                <Button
                    type="button"
                    size="small"
                    variant="ghost"
                    aria-expanded={open}
                    aria-controls={open ? threadId : undefined}
                    onClick={() => {
                        create.reset();
                        setOpen(current => !current);
                    }}
                >
                    {commentCountLabel(comments.length)}
                </Button>
            </div>
            {open && (
                <div id={threadId} className={styles.thread}>
                    {comments.length > 0 && (
                        <ol className={styles.commentList} aria-label="Comments">
                            {comments.map(comment => (
                                <CommentItem
                                    key={comment.id}
                                    snapshot={snapshot}
                                    comment={comment}
                                    writable={writable}
                                />
                            ))}
                        </ol>
                    )}
                    {canComment && (
                        <CommentBox
                            boardId={snapshot.board_id}
                            label="Add a comment"
                            submitLabel="Add comment"
                            initialBody=""
                            initialMentions={NO_MENTIONS}
                            pending={create.isPending}
                            error={create.error}
                            onSubmit={(body, named, done) =>
                                create.mutate({ cardId: top.id, body, mentions: named }, { onSuccess: done })
                            }
                        />
                    )}
                </div>
            )}
        </>
    );
}
