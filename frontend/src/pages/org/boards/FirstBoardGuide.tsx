import { useId } from "react";
import { AddBoardTile } from "../../../components/board/AddBoardTile/AddBoardTile";
import { NEW_BOARD_NO_TEAM, newBoardPath } from "../../../domain/board/paths";
import type { OrganisationResponse } from "../../../types/api";
import styles from "./OrgBoardsPage.module.css";

interface FirstBoardGuideProps {
    organisation: OrganisationResponse;
    canCreate: boolean;
}

export function FirstBoardGuide({ organisation, canCreate }: FirstBoardGuideProps) {
    const titleId = useId();

    return (
        <section aria-labelledby={titleId} className={styles.firstBoard}>
            <h2 id={titleId} className={styles.sectionTitle}>
                No boards yet
            </h2>
            {canCreate ? (
                <>
                    <p className={styles.hint}>Pick a template, give it a name, then share the link with your team.</p>
                    <div className={styles.firstTile}>
                        <AddBoardTile
                            href={newBoardPath(organisation.slug, NEW_BOARD_NO_TEAM)}
                            label="Create your first board"
                        />
                    </div>
                </>
            ) : (
                <p className={styles.hint}>Boards shared with you will show up here.</p>
            )}
        </section>
    );
}
