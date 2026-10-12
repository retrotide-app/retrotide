import { useParams } from "react-router";
import { sharedBoardPath } from "../../domain/board/paths";
import { BoardSummaryView } from "./BoardSummaryView";

export function SharedBoardSummaryPage() {
    const { boardId = "" } = useParams();

    return <BoardSummaryView key={boardId} boardId={boardId} boardLink={sharedBoardPath(boardId)} />;
}
