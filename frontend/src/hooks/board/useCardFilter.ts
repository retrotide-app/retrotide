import { useState } from "react";
import { type CardFilter, type CardSort, sortInForce } from "../../domain/board/cardFilter";
import type { SortMode } from "../../types/api";

export interface CardFilterState {
    filter: CardFilter;
    search: (query: string) => void;
    sortBy: (sort: CardSort) => void;
    showAuthor: (authorId: string | null) => void;
}

const SEED_RANGE = 2 ** 31;

export function useCardFilter(boardSort: SortMode, enforced: boolean): CardFilterState {
    const [query, setQuery] = useState("");
    const [chosenSort, setChosenSort] = useState<CardSort | null>(null);
    const [authorId, setAuthorId] = useState<string | null>(null);
    const [seed, setSeed] = useState(0);

    function sortBy(sort: CardSort) {
        if (enforced) {
            return;
        }

        if (sort === "random") {
            setSeed(Math.floor(Math.random() * SEED_RANGE));
        }

        setChosenSort(sort);
    }

    return {
        filter: { sort: sortInForce(chosenSort, boardSort, enforced), query, authorId, seed },
        search: setQuery,
        sortBy,
        showAuthor: setAuthorId,
    };
}
