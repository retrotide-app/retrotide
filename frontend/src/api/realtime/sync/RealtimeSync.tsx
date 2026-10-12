import { useEffect } from "react";
import type { Viewer } from "../../../types/app";
import { ensureRealtimePipeline } from "../pipeline";
import { closeRealtimeSocket, openRealtimeSocket } from "../socket";

export interface RealtimeSyncProps {
    viewer: Viewer | null;
}

export function RealtimeSync({ viewer }: RealtimeSyncProps): null {
    const sessionKey = viewer === null ? null : `${viewer.kind}:${viewer.id}`;

    useEffect(() => {
        if (sessionKey === null) {
            return;
        }

        ensureRealtimePipeline();
        openRealtimeSocket(sessionKey);

        return () => {
            closeRealtimeSocket(sessionKey);
        };
    }, [sessionKey]);

    return null;
}
