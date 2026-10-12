import { addReaction, pinCard, removeReaction } from "../../api/endpoints/cardPolish";
import type { ReactionKey } from "../../types/api";
import { useBoardCommand } from "./boardCommand";

export interface CardReaction {
    cardId: string;
    reaction: ReactionKey;
}

export interface CardPin {
    cardId: string;
    pinned: boolean;
}

export function useAddReaction(boardId: string) {
    return useBoardCommand(boardId, ({ cardId, reaction }: CardReaction, opId) =>
        addReaction(boardId, cardId, reaction, { op_id: opId }),
    );
}

export function useRemoveReaction(boardId: string) {
    return useBoardCommand(boardId, ({ cardId, reaction }: CardReaction, opId) =>
        removeReaction(boardId, cardId, reaction, { op_id: opId }),
    );
}

export function usePinCard(boardId: string) {
    return useBoardCommand(boardId, ({ cardId, pinned }: CardPin, opId) =>
        pinCard(boardId, cardId, { op_id: opId, pinned }),
    );
}
