import { useState } from "react";
import { REALTIME_EVENTS } from "../../api/realtime/events";
import { useRealtimeEvent } from "../../api/realtime/useRealtime";

interface Presence {
    boardId: string;
    online: ReadonlySet<string>;
}

export function useBoardPresence(boardId: string): ReadonlySet<string> | null {
    const [presence, setPresence] = useState<Presence | null>(null);

    useRealtimeEvent(REALTIME_EVENTS.BOARD_PRESENCE, event => {
        if (event.data.board_id !== boardId) {
            return;
        }

        setPresence({ boardId, online: new Set(event.data.viewers.map(viewer => viewer.participant_id)) });
    });

    return presence !== null && presence.boardId === boardId ? presence.online : null;
}
