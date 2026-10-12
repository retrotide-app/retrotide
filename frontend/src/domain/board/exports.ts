import type { BoardExportFormat } from "../../types/api";

export interface ExportFormatOption {
    format: BoardExportFormat;
    label: string;
}

export const BOARD_EXPORT_FORMATS: readonly ExportFormatOption[] = [
    { format: "csv", label: "CSV" },
    { format: "markdown", label: "Markdown" },
    { format: "json", label: "JSON" },
];
