import type { BoardCommandRequest } from "./board";

export type MediaKind = "card" | "background";

export interface MediaView {
    id: string;
    path: string;
    width: number;
    height: number;
}

export interface CardImageRequest extends BoardCommandRequest {
    image_id: string | null;
}

export interface BoardBackgroundRequest extends BoardCommandRequest {
    image_id: string | null;
}
