export interface GuestClaimBoard {
    board_id: string;
    title: string;
    display_name: string;
}

export interface GuestClaimOfferResponse {
    boards: GuestClaimBoard[];
}

export interface ClaimGuestBoardsRequest {
    board_ids: string[];
}

export interface ClaimGuestBoardsResponse {
    claimed: string[];
}
