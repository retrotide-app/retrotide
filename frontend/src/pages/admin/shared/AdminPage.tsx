import { type ReactNode, useId } from "react";
import { PageHeader } from "../../../components/PageHeader/PageHeader";
import { usePageTitle } from "../../../hooks/usePageTitle";
import styles from "./AdminPage.module.css";

interface AdminPageProps {
    title: string;
    description?: ReactNode;
    actions?: ReactNode;
    children?: ReactNode;
}

export function AdminPage({ title, description, actions, children }: AdminPageProps) {
    const titleId = useId();

    usePageTitle(title);

    return (
        <section aria-labelledby={titleId} className={styles.page}>
            <PageHeader
                titleId={titleId}
                title={title}
                description={description}
                headingLevel={2}
                actions={actions}
                className={styles.header}
            />
            {children}
        </section>
    );
}
