import type { BoardSnapshot, CardView } from "../../types/api";
import { type BoardColumnModel, boardColumns } from "./selectors";
import { type CardStack, stackCards, stackOf } from "./stacks";
import { votesLabel } from "./summary";
import { stackVotes } from "./votes";

const LINE_BREAKS = /\s*\n\s*/g;

function flatText(body: string): string {
    return body.replace(LINE_BREAKS, " ").trim();
}

function cardLine(body: string, total: number | null): string {
    if (total === null || total <= 0) {
        return `- ${flatText(body)}`;
    }

    return `- ${flatText(body)} (${votesLabel(total)})`;
}

function stackLines(snapshot: BoardSnapshot, stack: CardStack): string[] {
    const total = stackVotes(stackCards(stack), snapshot.me).total;

    return [cardLine(stack.card.body, total), ...stack.merged.map(card => `  - ${flatText(card.body)}`)];
}

function columnLines(snapshot: BoardSnapshot, model: BoardColumnModel): string[] {
    return [`## ${flatText(model.column.title)}`, ...model.stacks.flatMap(stack => stackLines(snapshot, stack))];
}

export function cardClipboardText(snapshot: BoardSnapshot, card: CardView): string {
    const stack = stackOf(snapshot.cards, card.id);

    if (stack === null || stack.card.id !== card.id) {
        return cardLine(card.body, card.votes.total);
    }

    return stackLines(snapshot, stack).join("\n");
}

export function columnClipboardText(snapshot: BoardSnapshot, columnId: string): string {
    const model = boardColumns(snapshot).find(held => held.column.id === columnId);

    if (model === undefined) {
        return "";
    }

    return columnLines(snapshot, model).join("\n");
}

export function boardClipboardText(snapshot: BoardSnapshot): string {
    const columns = boardColumns(snapshot).map(model => columnLines(snapshot, model).join("\n"));

    return [`# ${flatText(snapshot.board.title)}`, ...columns].join("\n\n");
}
