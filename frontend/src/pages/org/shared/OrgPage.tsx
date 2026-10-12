import { type ReactNode, useId } from "react";
import { PageHeader } from "../../../components/PageHeader/PageHeader";
import { usePageTitle } from "../../../hooks/usePageTitle";
import styles from "./OrgPage.module.css";

interface OrgPageProps {
    title: string;
    description?: ReactNode;
    badge?: ReactNode;
    status?: ReactNode;
    children?: ReactNode;
}

export function OrgPage({ title, description, badge, status, children }: OrgPageProps) {
    const titleId = useId();

    usePageTitle(title);

    return (
        <section aria-labelledby={titleId} className={styles.page}>
            <PageHeader titleId={titleId} title={title} description={description} badge={badge} actions={status} />
            {children}
        </section>
    );
}
