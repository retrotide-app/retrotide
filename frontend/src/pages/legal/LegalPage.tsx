import { useParams } from "react-router";
import { MarkdownContent } from "../../components/MarkdownContent/MarkdownContent";
import { MarketingLayout } from "../../components/marketing/MarketingLayout/MarketingLayout";
import { usePageTitle } from "../../hooks/usePageTitle";
import { NotFoundPage } from "../notfound/NotFoundPage";
import { LEGAL_DOCUMENTS, type LegalDocument } from "./legalDocuments";
import styles from "./LegalPage.module.css";

export function LegalPage() {
    const { page } = useParams<"page">();
    const legalDocument = LEGAL_DOCUMENTS.get(page ?? "");

    if (legalDocument === undefined) {
        return (
            <MarketingLayout>
                <NotFoundPage />
            </MarketingLayout>
        );
    }

    return (
        <MarketingLayout>
            <LegalArticle title={legalDocument.title} source={legalDocument.source} />
        </MarketingLayout>
    );
}

function LegalArticle({ title, source }: LegalDocument) {
    usePageTitle(title);

    return (
        <article className={styles.article}>
            <MarkdownContent source={source} />
        </article>
    );
}
