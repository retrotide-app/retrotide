import { createContext, useContext, useEffect, useMemo, useRef, useState } from "react";

export interface Arrivals {
    isNew: (key: string) => boolean;
}

const NO_ARRIVALS: Arrivals = { isNew: () => false };

export const ArrivalsContext = createContext<Arrivals>(NO_ARRIVALS);

export function useArrivalTracker(keys: readonly string[]): Arrivals {
    const [initial] = useState(() => new Set(keys));
    const seen = useRef(new Set<string>());

    useEffect(() => {
        for (const key of keys) {
            seen.current.add(key);
        }
    }, [keys]);

    return useMemo(() => ({ isNew: (key: string) => !initial.has(key) && !seen.current.has(key) }), [initial]);
}

export function useArrival(key: string): boolean {
    const { isNew } = useContext(ArrivalsContext);
    const [arriving] = useState(() => isNew(key));

    return arriving;
}
