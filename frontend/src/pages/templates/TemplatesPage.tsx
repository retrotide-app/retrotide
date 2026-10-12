import { ButtonLink } from "../../components/Button/ButtonLink";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { MarketingHero } from "../../components/marketing/MarketingHero/MarketingHero";
import { MarketingLayout } from "../../components/marketing/MarketingLayout/MarketingLayout";
import { MarketingSection } from "../../components/marketing/MarketingSection/MarketingSection";
import { TemplateCard } from "../../components/marketing/TemplateCard/TemplateCard";
import { publicTemplates, templatePath } from "../../domain/templates";
import { useBoardTemplates } from "../../hooks/queries/boards";
import { usePageTitle } from "../../hooks/usePageTitle";
import styles from "./TemplatesPage.module.css";

const PAGE_TITLE = "Retrospective templates";

export function TemplatesPage() {
    usePageTitle(PAGE_TITLE);

    const templates = useBoardTemplates();
    const shown = templates.data === undefined ? [] : publicTemplates(templates.data.templates);

    return (
        <MarketingLayout>
            <MarketingHero
                title={PAGE_TITLE}
                eyebrow="Templates"
                lede="Start your next retro from a format teams already know. Every template is free, and once the board is open you can rename, recolour, add or remove its columns."
            >
                <ButtonLink to="/register" variant="primary" size="large">
                    Start for free
                </ButtonLink>
            </MarketingHero>

            <MarketingSection title="All templates">
                {templates.error !== null && (
                    <LoadFailure message={templates.error.message} onRetry={templates.refetch} />
                )}
                {templates.data === undefined && templates.error === null && (
                    <p className="loading">Loading templates...</p>
                )}
                {templates.data !== undefined && shown.length === 0 && <p>No templates are available.</p>}
                {shown.length > 0 && (
                    <ul className={styles.grid} aria-label="Templates">
                        {shown.map(template => (
                            <li key={template.key}>
                                <TemplateCard template={template} href={templatePath(template.key)} />
                            </li>
                        ))}
                    </ul>
                )}
            </MarketingSection>
        </MarketingLayout>
    );
}
