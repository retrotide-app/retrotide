import { useParams } from "react-router";
import { TextLink } from "../../components/TextLink/TextLink";
import { boardSettingsPath, boardSummaryPath } from "../../domain/board/paths";
import { isDeleted, organisationHomePath, teamPath } from "../../domain/organisations";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import type { BoardView } from "../../types/api";
import { DeletedOrganisation } from "../org/shared/DeletedOrganisation";
import { LiveBoard } from "./LiveBoard";

const PAGE_TITLE = "Board";

export function BoardPage() {
    const { boardId = "" } = useParams();
    const { current } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    const { slug } = current;

    function backLink(board: BoardView) {
        if (board.team_id === undefined) {
            return <TextLink to={organisationHomePath(slug)}>Back to boards</TextLink>;
        }

        return <TextLink to={teamPath(slug, board.team_id)}>Back to team</TextLink>;
    }

    return (
        <LiveBoard
            key={`${current.id}/${boardId}`}
            boardId={boardId}
            backLink={backLink}
            settingsPath={boardSettingsPath(slug, boardId)}
            summaryPath={boardSummaryPath(slug, boardId)}
        />
    );
}
