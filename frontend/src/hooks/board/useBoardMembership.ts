import { MEMBERS_ONLY_REFUSALS, refusedWith } from "../../domain/refusals";
import type { PersonView } from "../../types/api";
import { useBoardPeople } from "../queries/actionItems";

export interface BoardMembership {
    people: PersonView[] | null;
    member: boolean;
    failure: Error | null;
    retry: () => Promise<unknown>;
}

export function useBoardMembership(boardId: string, wanted: boolean): BoardMembership {
    const people = useBoardPeople(boardId, wanted);
    const membersOnly = refusedWith(people.error, MEMBERS_ONLY_REFUSALS);

    return {
        people: people.data?.people ?? null,
        member: people.data !== undefined,
        failure: membersOnly ? null : people.error,
        retry: people.refetch,
    };
}
