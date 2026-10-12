import { type ChangeEvent, type SubmitEvent, useState } from "react";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FormField } from "../../components/FormField/FormField";
import { Input } from "../../components/Input/Input";
import { Panel } from "../../components/Panel/Panel";
import { TextArea } from "../../components/TextArea/TextArea";
import { Toast } from "../../components/Toast/Toast";
import { useImportCsv } from "../../hooks/mutations/boardManagement";
import type { BoardSnapshot, CsvImportResponse } from "../../types/api";
import { countOf, type Plural } from "../../utils/chart";
import { ActionFailure } from "../org/shared/ActionFailure";
import sectionStyles from "../org/shared/OrgSection.module.css";

interface CsvImportSectionProps {
    snapshot: BoardSnapshot;
}

const CSV_MISSING = "Paste a CSV or choose a CSV file.";

const CARD_NOUN: Plural = { one: "card", other: "cards" };

const COLUMN_NOUN: Plural = { one: "new column", other: "new columns" };

function importedLabel({ cards_created: cards, columns_created: columns }: CsvImportResponse): string {
    if (columns === 0) {
        return `Added ${countOf(cards, CARD_NOUN)}.`;
    }

    return `Added ${countOf(cards, CARD_NOUN)} and ${countOf(columns, COLUMN_NOUN)}.`;
}

export function CsvImportSection({ snapshot }: CsvImportSectionProps) {
    const importer = useImportCsv(snapshot.board_id);
    const [csv, setCsv] = useState("");
    const [problem, setProblem] = useState<string | null>(null);
    const [imported, setImported] = useState<CsvImportResponse | null>(null);

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();
        setImported(null);

        if (csv.trim() === "") {
            importer.reset();
            setProblem(CSV_MISSING);
            return;
        }

        setProblem(null);
        importer.mutate(csv, {
            onSuccess: answer => {
                setImported(answer);
                setCsv("");
            },
        });
    }

    function handleFile(e: ChangeEvent<HTMLInputElement>) {
        const file = e.target.files?.[0];

        if (file === undefined) {
            return;
        }

        file.text()
            .then(text => {
                setProblem(null);
                setCsv(text);
            })
            .catch((error: unknown) => {
                setProblem(error instanceof Error ? error.message : String(error));
            });
    }

    return (
        <Panel
            title="Import cards"
            headingLevel={2}
            description="Add cards from a spreadsheet. The first row holds the column titles, and each cell below it becomes a card in that column."
        >
            <form className={sectionStyles.form} onSubmit={handleSubmit} aria-label="Import cards" noValidate>
                {problem !== null && <ErrorBanner message={problem} takeFocus />}
                {problem === null && importer.error !== null && <ActionFailure error={importer.error} takeFocus />}
                {imported !== null && (
                    <Toast variant="success" onDismiss={() => setImported(null)}>
                        {importedLabel(imported)}
                    </Toast>
                )}
                <FormField
                    label="CSV file"
                    hint="A title that matches a column adds to it; any other title makes a new column."
                    layout="row"
                >
                    {control => <Input {...control} type="file" accept=".csv,text/csv" onChange={handleFile} />}
                </FormField>
                <FormField
                    label="CSV text"
                    hint="Up to 500 cards, and a board holds at most 20 columns. Empty cells are skipped."
                    layout="row"
                >
                    {control => (
                        <TextArea
                            {...control}
                            value={csv}
                            onChange={e => setCsv(e.target.value)}
                            rows={6}
                            placeholder={"Went well,To improve\nShipped on time,Flaky tests"}
                        />
                    )}
                </FormField>
                <div className={sectionStyles.actions}>
                    <Button type="submit" variant="primary" disabled={importer.isPending}>
                        Import cards
                    </Button>
                </div>
            </form>
        </Panel>
    );
}
