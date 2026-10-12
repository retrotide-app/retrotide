import { Fragment } from "react";
import { titleParts } from "../../../domain/board/mentions";
import type { PersonView } from "../../../types/api";
import styles from "./Mentions.module.css";

interface MentionTextProps {
    text: string;
    mentions: readonly PersonView[];
    id?: string;
}

export function MentionText({ text, mentions, id }: MentionTextProps) {
    return (
        <span id={id} className={styles.text} dir="auto">
            {titleParts(text, mentions).map((part, index) =>
                part.mention === null ? (
                    <Fragment key={index}>{part.text}</Fragment>
                ) : (
                    <bdi key={index} className={styles.mention}>
                        {part.text}
                    </bdi>
                ),
            )}
        </span>
    );
}
