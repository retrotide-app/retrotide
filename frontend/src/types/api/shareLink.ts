import type { BoardCommandRequest } from "./board";

export type LinkAccess = "off" | "guests" | "accounts";

export interface JoinPreviewResponse {
    requires_password: boolean;
    requires_account: boolean;
    can_join: boolean;
}

export interface JoinBoardRequest {
    display_name?: string;
    password?: string;
}

export interface JoinBoardResponse {
    board_id: string;
    org_id?: string;
}

export interface BoardSharingResponse {
    share_slug: string;
    link_access: LinkAccess;
    has_password: boolean;
}

export interface SetLinkAccessRequest extends BoardCommandRequest {
    link_access: LinkAccess;
}

export interface SetBoardPasswordRequest extends BoardCommandRequest {
    password: string | null;
}

export interface SharingCommandResponse {
    version: number;
    replayed?: true;
    sharing?: BoardSharingResponse;
}
