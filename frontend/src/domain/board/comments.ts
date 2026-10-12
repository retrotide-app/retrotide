import type { CardView, CommentView } from "../../types/api";

export function commentsOnStack(comments: readonly CommentView[], stack: readonly CardView[]): CommentView[] {
    const cardIds = new Set(stack.map(card => card.id));

    return comments.filter(comment => cardIds.has(comment.card_id));
}

export function commentCountLabel(count: number): string {
    if (count === 0) {
        return "Comment";
    }

    return count === 1 ? "1 comment" : `${count} comments`;
}

export function isEdited(comment: CommentView): boolean {
    return comment.updated_at !== undefined && comment.updated_at !== comment.created_at;
}
