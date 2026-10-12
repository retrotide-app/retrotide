import { useEffect, useEffectEvent, useRef, useState } from "react";
import { useLocation, useParams } from "react-router";
import printStyles from "../../components/board/print/BoardPrint.module.css";
import { BoardPrintView } from "../../components/board/print/BoardPrintView";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { TextLink } from "../../components/TextLink/TextLink";
import { sharedBoardPath } from "../../domain/board/paths";
import { boardImageFileName, type PrintAction, printAction } from "../../domain/board/print";
import { useBoard } from "../../hooks/queries/boards";
import { usePageTitle } from "../../hooks/usePageTitle";
import { exportNodeAsPng } from "../../platform/imageExport";
import { printPage } from "../../platform/print";
import { errorMessage } from "../../utils/errorMessage";

const PAGE_TITLE = "Print board";

const PRINT_FAILED = "The board could not be printed. Use the browser's own menu to print it.";

const PNG_FAILED = "The board could not be saved as a PNG. Try again.";

export function BoardPrintPage() {
    const { boardId = "" } = useParams();
    const { search } = useLocation();
    const { data: snapshot, error, refetch } = useBoard(boardId);
    const sheetRef = useRef<HTMLElement>(null);
    const handled = useRef(false);
    const [failure, setFailure] = useState<string | null>(null);
    const [exporting, setExporting] = useState(false);

    const action = printAction(search);
    const ready = snapshot !== undefined;

    usePageTitle(snapshot === undefined ? PAGE_TITLE : snapshot.board.title);

    function runPrint() {
        setFailure(null);

        try {
            printPage();
        } catch (thrown) {
            setFailure(errorMessage(thrown, PRINT_FAILED));
        }
    }

    function runPng(title: string) {
        const node = sheetRef.current;

        if (node === null) {
            return;
        }

        setFailure(null);
        setExporting(true);

        exportNodeAsPng(node, boardImageFileName(title, new Date()))
            .catch((thrown: unknown) => setFailure(errorMessage(thrown, PNG_FAILED)))
            .finally(() => setExporting(false));
    }

    const perform = useEffectEvent((then: PrintAction) => {
        if (handled.current || snapshot === undefined) {
            return;
        }

        handled.current = true;

        if (then === "print") {
            runPrint();
            return;
        }

        runPng(snapshot.board.title);
    });

    useEffect(() => {
        if (!ready || action === null) {
            return;
        }

        const id = window.setTimeout(() => perform(action), 0);

        return () => {
            window.clearTimeout(id);
        };
    }, [ready, action]);

    if (snapshot === undefined) {
        return (
            <main className={printStyles.page}>
                <div className={printStyles.screenOnly}>
                    {error !== null && <LoadFailure message={error.message} onRetry={refetch} />}
                    {error === null && <p className="loading">Loading board...</p>}
                </div>
            </main>
        );
    }

    return (
        <main className={printStyles.page}>
            <div className={printStyles.screenOnly}>
                <div className={printStyles.toolbar}>
                    <TextLink to={sharedBoardPath(boardId)}>Back to board</TextLink>
                    <Button type="button" variant="primary" onClick={() => runPrint()}>
                        Print or save as PDF
                    </Button>
                    <Button type="button" disabled={exporting} onClick={() => runPng(snapshot.board.title)}>
                        Download PNG
                    </Button>
                </div>
                {failure !== null && <ErrorBanner message={failure} takeFocus />}
            </div>
            <BoardPrintView ref={sheetRef} snapshot={snapshot} />
        </main>
    );
}
