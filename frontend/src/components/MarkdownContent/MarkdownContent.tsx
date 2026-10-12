import { useMemo } from "react";
import { renderMarkdown } from "../../platform/markdown";
import styles from "./MarkdownContent.module.css";

interface MarkdownContentProps {
    source: string;
}

export function MarkdownContent({ source }: MarkdownContentProps) {
    const html = useMemo(() => renderMarkdown(source), [source]);

    return <div className={styles.content} dangerouslySetInnerHTML={{ __html: html }} />;
}
