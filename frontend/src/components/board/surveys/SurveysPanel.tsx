import { useState } from "react";
import { surveyActions } from "../../../domain/board/capabilities";
import type { BoardSnapshot } from "../../../types/api";
import { Button } from "../../Button/Button";
import { Panel } from "../../Panel/Panel";
import { NewSurveyDialog } from "./NewSurveyDialog";
import { SurveyCard } from "./SurveyCard";
import styles from "./Surveys.module.css";

interface SurveysPanelProps {
    snapshot: BoardSnapshot;
    writable: boolean;
}

export function SurveysPanel({ snapshot, writable }: SurveysPanelProps) {
    const [creating, setCreating] = useState(false);
    const actions = surveyActions(snapshot);
    const { board_id: boardId, surveys } = snapshot;
    const manage = writable && actions.manage;
    const respond = writable && actions.respond && !snapshot.settings.board_locked;

    if (surveys.length === 0 && !actions.manage) {
        return null;
    }

    return (
        <Panel
            title="Surveys"
            headingLevel={2}
            description="Health checks and quick questions for the team. Results stay hidden until a survey is closed."
        >
            {manage && (
                <div className={styles.toolbar}>
                    <Button type="button" variant="primary" size="small" onClick={() => setCreating(true)}>
                        New survey
                    </Button>
                </div>
            )}
            {surveys.length === 0 ? (
                <p className={styles.empty}>No surveys yet. Start a health check or ask the team a quick question.</p>
            ) : (
                <ul className={styles.grid}>
                    {surveys.map(survey => (
                        <li key={survey.id}>
                            <SurveyCard boardId={boardId} survey={survey} canRespond={respond} canManage={manage} />
                        </li>
                    ))}
                </ul>
            )}
            {creating && <NewSurveyDialog boardId={boardId} onClose={() => setCreating(false)} />}
        </Panel>
    );
}
