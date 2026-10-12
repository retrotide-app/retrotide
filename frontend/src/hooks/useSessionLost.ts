import { useSyncExternalStore } from "react";
import { isSessionLost, subscribeSessionLost } from "../api/sessionLost";
import { useAuth } from "./useAuth";

export function useSessionLost(): boolean {
    const lost = useSyncExternalStore(subscribeSessionLost, isSessionLost);
    const { user } = useAuth();

    return user !== null && lost;
}
