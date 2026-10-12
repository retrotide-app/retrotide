import type { QueryClient, QueryKey } from "@tanstack/react-query";
import { errorStatus } from "../../utils/errorMessage";

const SAVED_BUT_FAILED_STATUS = 500;

function mayHaveChanged(error: Error | null, outdatedOn: readonly number[]): boolean {
    if (error === null) {
        return true;
    }

    const status = errorStatus(error);

    return status === SAVED_BUT_FAILED_STATUS || (status !== null && outdatedOn.includes(status));
}

export async function refreshWhenChanged(
    qc: QueryClient,
    error: Error | null,
    keys: readonly QueryKey[],
    outdatedOn: readonly number[] = [],
): Promise<void> {
    if (!mayHaveChanged(error, outdatedOn)) {
        return;
    }

    await Promise.all(keys.map(queryKey => qc.invalidateQueries({ queryKey })));
}
