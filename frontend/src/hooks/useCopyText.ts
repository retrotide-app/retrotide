import { useCallback, useState } from "react";
import { copyText } from "../platform/clipboard";

export interface CopyText {
    copy: (text: string) => void;
    copied: boolean;
    error: Error | null;
    reset: () => void;
}

function asError(failure: unknown): Error {
    return failure instanceof Error ? failure : new Error(String(failure), { cause: failure });
}

export function useCopyText(): CopyText {
    const [copied, setCopied] = useState(false);
    const [error, setError] = useState<Error | null>(null);

    const copy = useCallback((text: string) => {
        setCopied(false);
        setError(null);

        copyText(text)
            .then(() => setCopied(true))
            .catch((failure: unknown) => setError(asError(failure)));
    }, []);

    const reset = useCallback(() => {
        setCopied(false);
        setError(null);
    }, []);

    return { copy, copied, error, reset };
}
