import { BOARD_STAGES, STAGE_INFO, STAGE_LABELS } from "../../domain/board/stages";
import type { BoardView } from "../../types/api";
import { StepProgress } from "../StepProgress/StepProgress";
import styles from "./StageBar.module.css";

interface StageBarProps {
    board: BoardView;
}

export function StageBar({ board }: StageBarProps) {
    const { stage } = board;

    return (
        <div className={styles.bar} role="region" aria-label="Retro stage">
            <StepProgress label="Retro stages" steps={STAGE_LABELS} current={BOARD_STAGES.indexOf(stage)} compact />
            <p className={styles.purpose} aria-live="polite">
                {STAGE_INFO[stage].purpose}
            </p>
        </div>
    );
}
