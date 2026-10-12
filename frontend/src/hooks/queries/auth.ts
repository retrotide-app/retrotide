import { useQuery } from "@tanstack/react-query";
import { getInvitation, getSession } from "../../api/endpoints/auth";
import { queryKeys } from "../../api/queryKeys";
import type { InvitationResponse, SessionResponse } from "../../types/api";
import { queryState, type QueryState } from "./queryState";

export function useSession(): QueryState<SessionResponse> {
    const query = useQuery({ queryKey: queryKeys.auth.session(), queryFn: getSession });

    return queryState(query);
}

export function useInvitation(token: string): QueryState<InvitationResponse> {
    const invitation = useQuery({
        queryKey: queryKeys.auth.invitation(token),
        queryFn: () => getInvitation({ token }),
        enabled: token !== "",
    });

    return queryState(invitation);
}
