import type { ClaimGuestBoardsRequest, ClaimGuestBoardsResponse, GuestClaimOfferResponse } from "../../types/api";
import { apiDelete, apiFetch, apiPost } from "../client";

const GUEST_CLAIM_PATH = "/auth/guest-claim";

export async function getGuestClaimOffer(): Promise<GuestClaimOfferResponse> {
    return apiFetch<GuestClaimOfferResponse>(GUEST_CLAIM_PATH);
}

export async function claimGuestBoards(body: ClaimGuestBoardsRequest): Promise<ClaimGuestBoardsResponse> {
    return apiPost<ClaimGuestBoardsResponse, ClaimGuestBoardsRequest>(GUEST_CLAIM_PATH, body);
}

export async function declineGuestClaim(): Promise<void> {
    return apiDelete<void>(GUEST_CLAIM_PATH);
}
