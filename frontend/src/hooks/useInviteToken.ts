import { useEffect, useState } from "react";
import { clearLocationToken, readLocationToken } from "../platform/location";
import { clearPendingInvite, readPendingInvite, writePendingInvite } from "../platform/pendingInvite";

export interface InviteToken {
    token: string;
    forget: () => void;
}

interface InitialToken {
    token: string;
    fromLink: boolean;
}

function initialToken(): InitialToken {
    const fromLink = readLocationToken();

    if (fromLink !== "") {
        return { token: fromLink, fromLink: true };
    }

    return { token: readPendingInvite() ?? "", fromLink: false };
}

export function useInviteToken(): InviteToken {
    const [initial] = useState(initialToken);

    useEffect(() => {
        if (!initial.fromLink) {
            return;
        }

        writePendingInvite(initial.token);
        clearLocationToken();
    }, [initial]);

    return { token: initial.token, forget: clearPendingInvite };
}

export function useHasPendingInvite(): boolean {
    const [pending] = useState(() => readPendingInvite() !== null);

    return pending;
}
