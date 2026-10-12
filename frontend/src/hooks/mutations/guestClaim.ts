import { useMutation, useQueryClient } from "@tanstack/react-query";
import { claimGuestBoards, declineGuestClaim } from "../../api/endpoints/guestClaim";
import { queryKeys } from "../../api/queryKeys";
import type { GuestClaimOfferResponse } from "../../types/api";
import { refreshWhenChanged } from "./refresh";

const CONFLICT_STATUS = 409;

const NOTHING_OFFERED: GuestClaimOfferResponse = { boards: [] };

export function useClaimGuestBoards() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: (boardIds: string[]) => claimGuestBoards({ board_ids: boardIds }),
        onSuccess: async () => {
            qc.setQueriesData<GuestClaimOfferResponse>({ queryKey: queryKeys.auth.guestClaims }, NOTHING_OFFERED);

            await Promise.all([
                qc.invalidateQueries({ queryKey: queryKeys.boardLive.all }),
                qc.invalidateQueries({ queryKey: queryKeys.boards.all }),
            ]);
        },
        onError: error => refreshWhenChanged(qc, error, [queryKeys.auth.guestClaims], [CONFLICT_STATUS]),
    });
}

export function useDeclineGuestClaim() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: declineGuestClaim,
        onSuccess: () => {
            qc.setQueriesData<GuestClaimOfferResponse>({ queryKey: queryKeys.auth.guestClaims }, NOTHING_OFFERED);
        },
    });
}
