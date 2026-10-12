import { useMemo } from "react";
import { boardItemKeys } from "../../domain/board/selectors";
import { ArrivalsContext, useArrivalTracker } from "../../hooks/board/useArrivals";
import { useBoardLayout } from "../../hooks/board/useBoardLayout";
import type { CardFocus } from "../../hooks/board/useCardFocus";
import { useCardFilter } from "../../hooks/board/useCardFilter";
import type { BoardSnapshot } from "../../types/api";
import { BoardColumns } from "./BoardColumns";
import { BoardToolbar } from "./BoardToolbar";
import styles from "./BoardViews.module.css";
import { BoardListView } from "./list/BoardListView";

interface BoardViewsProps {
    snapshot: BoardSnapshot;
    writable: boolean;
    cardFocus: CardFocus;
}

export function BoardViews({ snapshot, writable, cardFocus }: BoardViewsProps) {
    const { layout, choose } = useBoardLayout();
    const cards = useCardFilter(snapshot.settings.sort_mode, snapshot.settings.sort_enforced);
    const keys = useMemo(() => boardItemKeys(snapshot), [snapshot]);
    const arrivals = useArrivalTracker(keys);

    return (
        <ArrivalsContext.Provider value={arrivals}>
            <div className={styles.views}>
                <BoardToolbar
                    snapshot={snapshot}
                    writable={writable}
                    cards={cards}
                    layout={layout}
                    onLayoutChange={choose}
                />
                {layout === "list" ? (
                    <BoardListView
                        snapshot={snapshot}
                        writable={writable}
                        cardFocus={cardFocus}
                        filter={cards.filter}
                    />
                ) : (
                    <BoardColumns snapshot={snapshot} writable={writable} cardFocus={cardFocus} filter={cards.filter} />
                )}
            </div>
        </ArrivalsContext.Provider>
    );
}
