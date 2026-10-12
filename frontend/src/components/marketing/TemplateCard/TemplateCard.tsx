import { Link } from "react-router";
import { columnCountLabel } from "../../../domain/templates";
import type { TemplateResponse } from "../../../types/api";
import colours from "../../board/boardColours.module.css";
import styles from "./TemplateCard.module.css";

interface TemplateCardProps {
    template: TemplateResponse;
    href: string;
}

export function TemplateCard({ template, href }: TemplateCardProps) {
    return (
        <article className={styles.card}>
            <span className={styles.strip} aria-hidden="true">
                {template.columns.map(column => (
                    <span key={column.title} className={`${styles.swatch} ${colours[column.colour]}`} />
                ))}
            </span>
            <h3 className={styles.name} dir="auto">
                <Link to={href} className={styles.link}>
                    <bdi>{template.name}</bdi>
                </Link>
            </h3>
            <p className={styles.description} dir="auto">
                {template.description}
            </p>
            <p className={styles.count}>{columnCountLabel(template.columns.length)}</p>
        </article>
    );
}
