import { useQuery } from "@tanstack/react-query";
import { getGuestClaimOffer } from "../../api/endpoints/guestClaim";
import { queryKeys } from "../../api/queryKeys";
import type { GuestClaimOfferResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useGuestClaimOffer(userId: string | null): QueryState<GuestClaimOfferResponse> {
    const offer = useQuery({
        queryKey: queryKeys.auth.guestClaim(userId ?? ""),
        queryFn: getGuestClaimOffer,
        enabled: userId !== null,
        staleTime: Infinity,
    });

    return queryState(offer);
}
