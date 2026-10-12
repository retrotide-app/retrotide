import { useLocation, useParams } from "react-router";
import { joinBoardPath, sharedBoardSummaryPath } from "../../domain/board/paths";
import { useRememberedJoinLink } from "../../hooks/board/useRememberedJoinLink";
import { LiveBoard, type SharedBoardLink } from "./LiveBoard";

export function SharedBoardPage() {
    const { boardId = "" } = useParams();
    const location = useLocation();

    const slug = useRememberedJoinLink(boardId, location.state);
    const link: SharedBoardLink = {
        rejoinPath: slug === null ? null : joinBoardPath(slug),
    };

    return <LiveBoard key={boardId} boardId={boardId} link={link} summaryPath={sharedBoardSummaryPath(boardId)} />;
}
