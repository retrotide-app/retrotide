import { useMutation } from "@tanstack/react-query";
import { setBoardBackground, setCardImage, uploadMedia } from "../../api/endpoints/media";
import type { MediaKind } from "../../types/api";
import { useBoardCommand } from "./boardCommand";

export interface MediaUpload {
    kind: MediaKind;
    file: File;
}

export interface CardImageChange {
    cardId: string;
    imageId: string | null;
}

export interface BackgroundChange {
    imageId: string | null;
}

export function useUploadMedia(boardId: string) {
    return useMutation({
        mutationFn: ({ kind, file }: MediaUpload) => uploadMedia(boardId, kind, file),
    });
}

export function useSetCardImage(boardId: string) {
    return useBoardCommand(boardId, ({ cardId, imageId }: CardImageChange, opId) =>
        setCardImage(boardId, cardId, { op_id: opId, image_id: imageId }),
    );
}

export function useSetBoardBackground(boardId: string) {
    return useBoardCommand(boardId, ({ imageId }: BackgroundChange, opId) =>
        setBoardBackground(boardId, { op_id: opId, image_id: imageId }),
    );
}
