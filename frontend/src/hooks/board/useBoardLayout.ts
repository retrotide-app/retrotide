import { useState, useSyncExternalStore } from "react";
import { isNarrowViewport, subscribeNarrowViewport } from "../../platform/viewport";

export type BoardLayout = "columns" | "list";

export interface BoardLayoutState {
    layout: BoardLayout;
    choose: (layout: BoardLayout) => void;
}

export function useBoardLayout(): BoardLayoutState {
    const narrow = useSyncExternalStore(subscribeNarrowViewport, isNarrowViewport);
    const [chosen, setChosen] = useState<BoardLayout | null>(null);

    const fitted: BoardLayout = narrow ? "list" : "columns";

    return { layout: chosen ?? fitted, choose: setChosen };
}
