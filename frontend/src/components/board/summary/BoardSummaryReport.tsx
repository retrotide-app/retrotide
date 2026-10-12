import {
    type BoardSummary,
    hiddenCardsLabel,
    participantsLabel,
    type SummaryColumn,
} from "../../../domain/board/summary";
import { formatDate } from "../../../utils/time";
import { Panel } from "../../Panel/Panel";
import styles from "./BoardSummary.module.css";
import { SummaryActionList } from "./SummaryActionList";
import { SummaryCardItem } from "./SummaryCardItem";

interface BoardSummaryReportProps {
    summary: BoardSummary;
}

interface SummaryColumnPanelProps {
    entry: SummaryColumn;
}

function SummaryColumnPanel({ entry }: SummaryColumnPanelProps) {
    const { column, stacks, hidden } = entry;

    return (
        <Panel title={column.title} headingLevel={2}>
            {stacks.length === 0 && hidden === 0 && <p className={styles.note}>No cards.</p>}
            {stacks.length > 0 && (
                <ol className={styles.cards} aria-label={`Cards in ${column.title}`}>
                    {stacks.map(stack => (
                        <li key={stack.top.card.id}>
                            <SummaryCardItem entry={stack.top} votes={stack.votes}>
                                {stack.merged.length > 0 && (
                                    <ul className={styles.merged} aria-label="Merged cards">
                                        {stack.merged.map(merged => (
                                            <li key={merged.card.id}>
                                                <SummaryCardItem entry={merged} votes={merged.votes} />
                                            </li>
                                        ))}
                                    </ul>
                                )}
                            </SummaryCardItem>
                        </li>
                    ))}
                </ol>
            )}
            {hidden > 0 && <p className={styles.note}>{hiddenCardsLabel(hidden)}</p>}
        </Panel>
    );
}

export function BoardSummaryReport({ summary }: BoardSummaryReportProps) {
    const created = formatDate(summary.createdAt);

    return (
        <div className={styles.report}>
            <p className={styles.facts}>
                {created !== "" && <span>Created {created}</span>}
                <span>{participantsLabel(summary.participants)}</span>
            </p>
            <div className={styles.columns}>
                {summary.columns.map(entry => (
                    <SummaryColumnPanel key={entry.column.id} entry={entry} />
                ))}
            </div>
            {summary.otherActions.length > 0 && (
                <Panel
                    title="Other actions"
                    headingLevel={2}
                    description="Action items that are not from a card you can see."
                >
                    <SummaryActionList label="Other actions" items={summary.otherActions} />
                </Panel>
            )}
        </div>
    );
}
