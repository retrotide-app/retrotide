import { useCopyText } from "../../hooks/useCopyText";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import styles from "./CommandBlock.module.css";

interface CommandBlockProps {
    command: string;
    label: string;
}

export function CommandBlock({ command, label }: CommandBlockProps) {
    const copy = useCopyText();

    return (
        <div className={styles.block}>
            <div className={styles.row}>
                <pre className={styles.code} aria-label={label} tabIndex={0}>
                    <code>{command}</code>
                </pre>
                <Button type="button" size="small" onClick={() => copy.copy(command)}>
                    {copy.copied ? "Copied" : "Copy"}
                </Button>
            </div>
            {copy.error !== null && <ErrorBanner message={copy.error.message} />}
        </div>
    );
}
