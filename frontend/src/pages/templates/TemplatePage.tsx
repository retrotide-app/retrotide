import { useParams } from "react-router";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { MarketingHero } from "../../components/marketing/MarketingHero/MarketingHero";
import { MarketingLayout } from "../../components/marketing/MarketingLayout/MarketingLayout";
import { MarketingSection } from "../../components/marketing/MarketingSection/MarketingSection";
import { TemplateColumns } from "../../components/marketing/TemplateColumns/TemplateColumns";
import { PageState } from "../../components/PageState/PageState";
import { TextLink } from "../../components/TextLink/TextLink";
import { TEMPLATES_PATH } from "../../domain/marketing";
import { columnCountLabel, isPublicTemplate, isTemplateNotFound, templateStartPath } from "../../domain/templates";
import { useBoardTemplate } from "../../hooks/queries/boards";
import type { QueryState } from "../../hooks/queries/queryState";
import { useAuth } from "../../hooks/useAuth";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import { usePageTitle } from "../../hooks/usePageTitle";
import type { TemplateResponse } from "../../types/api";
import { NotFoundPage } from "../notfound/NotFoundPage";
import styles from "./TemplatePage.module.css";

interface TemplateDetailProps {
    template: QueryState<TemplateResponse>;
}

function TemplateDetail({ template }: TemplateDetailProps) {
    const { user } = useAuth();
    const { current } = useCurrentOrg();
    usePageTitle(template.data?.name);

    if (template.error !== null) {
        return (
            <PageState title="This template could not load">
                <LoadFailure message={template.error.message} onRetry={template.refetch} />
            </PageState>
        );
    }

    if (template.data === undefined) {
        return <div className="loading">Loading...</div>;
    }

    const { name, description, columns } = template.data;
    const startPath = templateStartPath(user !== null, current?.slug ?? null);

    return (
        <>
            <MarketingHero title={name} eyebrow="Retrospective template" lede={description}>
                <ButtonLink to={startPath} variant="primary" size="large">
                    Use this template
                </ButtonLink>
            </MarketingHero>

            <MarketingSection
                title="Columns"
                intro={`${columnCountLabel(columns.length)}. Once the board is open you can rename, recolour, add or remove columns.`}
            >
                <TemplateColumns columns={columns} label={`${name} columns`} />
                <p className={styles.back}>
                    <TextLink to={TEMPLATES_PATH}>See all templates</TextLink>
                </p>
            </MarketingSection>
        </>
    );
}

export function TemplatePage() {
    const { key = "" } = useParams<"key">();
    const publicKey = isPublicTemplate(key) ? key : "";
    const template = useBoardTemplate(publicKey);

    if (publicKey === "" || isTemplateNotFound(template.error)) {
        return (
            <MarketingLayout>
                <NotFoundPage />
            </MarketingLayout>
        );
    }

    return (
        <MarketingLayout>
            <TemplateDetail template={template} />
        </MarketingLayout>
    );
}
