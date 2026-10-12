import { boardClipboardText } from "../../domain/board/clipboardText";
import { boardPrintPath } from "../../domain/board/print";
import { useBoardExportLinks } from "../../hooks/queries/exports";
import { useCopyText } from "../../hooks/useCopyText";
import type { BoardSnapshot } from "../../types/api";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { Menu, type MenuItem } from "../Menu/Menu";
import { Toast } from "../Toast/Toast";

interface BoardExportMenuProps {
    boardId: string;
    snapshot?: BoardSnapshot;
}

export function BoardExportMenu({ boardId, snapshot }: BoardExportMenuProps) {
    const links = useBoardExportLinks(boardId);
    const copy = useCopyText();

    const downloads: MenuItem[] = links.map(link => ({ label: link.label, href: link.href, download: true }));
    const pictures: MenuItem[] = [
        { label: "Print or save as PDF", href: boardPrintPath(boardId, "print"), newTab: true },
        { label: "Download PNG", href: boardPrintPath(boardId, "png"), newTab: true },
    ];
    const copying: MenuItem[] =
        snapshot === undefined
            ? []
            : [{ label: "Copy board to clipboard", onSelect: () => copy.copy(boardClipboardText(snapshot)) }];

    return (
        <>
            <Menu label="Export" items={[...downloads, ...pictures, ...copying]} />
            {copy.copied && (
                <Toast variant="success" onDismiss={copy.reset}>
                    Board copied to the clipboard.
                </Toast>
            )}
            {copy.error !== null && <ErrorBanner message={copy.error.message} />}
        </>
    );
}
