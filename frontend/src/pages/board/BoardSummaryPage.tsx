import { useParams } from "react-router";
import { boardPath } from "../../domain/board/paths";
import { isDeleted } from "../../domain/organisations";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import { DeletedOrganisation } from "../org/shared/DeletedOrganisation";
import { BoardSummaryView } from "./BoardSummaryView";

const PAGE_TITLE = "Board summary";

export function BoardSummaryPage() {
    const { boardId = "" } = useParams();
    const { current } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    return (
        <BoardSummaryView
            key={`${current.id}/${boardId}`}
            boardId={boardId}
            boardLink={boardPath(current.slug, boardId)}
        />
    );
}
