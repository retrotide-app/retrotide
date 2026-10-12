import { useId, useState } from "react";
import colours from "../../../components/board/boardColours.module.css";
import { Button } from "../../../components/Button/Button";
import { ButtonLink } from "../../../components/Button/ButtonLink";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { isDeleted } from "../../../domain/organisations";
import { newBoardFromTemplatePath, savedTemplateNote } from "../../../domain/savedTemplates";
import { useDeleteSavedTemplate } from "../../../hooks/mutations/savedTemplates";
import { useSavedTemplates } from "../../../hooks/queries/savedTemplates";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import type { OrganisationResponse, SavedTemplateResponse } from "../../../types/api";
import { ActionFailure } from "../shared/ActionFailure";
import { DeletedOrganisation } from "../shared/DeletedOrganisation";
import { OrgPage } from "../shared/OrgPage";
import styles from "./SavedTemplatesPage.module.css";

const PAGE_TITLE = "Templates";

interface SavedTemplatesHomeProps {
    organisation: OrganisationResponse;
}

interface SavedTemplateCardProps {
    organisation: OrganisationResponse;
    template: SavedTemplateResponse;
    onDelete: (template: SavedTemplateResponse) => void;
}

function SavedTemplateCard({ organisation, template, onDelete }: SavedTemplateCardProps) {
    const titleId = useId();

    return (
        <li className={styles.card} aria-labelledby={titleId}>
            <div className={styles.heading}>
                <h2 id={titleId} className={styles.name} dir="auto">
                    {template.name}
                </h2>
                <StatusTag tone="neutral">
                    <bdi>{savedTemplateNote(template)}</bdi>
                </StatusTag>
            </div>
            {template.description !== "" && (
                <p className={styles.description} dir="auto">
                    {template.description}
                </p>
            )}
            <p className="visually-hidden">{`Columns: ${template.columns.map(column => column.title).join(", ")}.`}</p>
            <div className={styles.board} aria-hidden="true">
                {template.columns.map((column, index) => (
                    <span key={`${index}:${column.title}`} className={`${styles.column} ${colours[column.colour]}`}>
                        <span className={styles.bar} />
                        <span className={styles.columnTitle} dir="auto">
                            {column.title}
                        </span>
                    </span>
                ))}
            </div>
            <div className={styles.actions}>
                <ButtonLink
                    to={newBoardFromTemplatePath(organisation.slug, template.id)}
                    variant="primary"
                    size="small"
                    aria-describedby={titleId}
                >
                    Use template
                </ButtonLink>
                {template.can_delete && (
                    <Button
                        type="button"
                        variant="ghost"
                        size="small"
                        aria-describedby={titleId}
                        onClick={() => onDelete(template)}
                    >
                        Delete
                    </Button>
                )}
            </div>
        </li>
    );
}

function SavedTemplatesHome({ organisation }: SavedTemplatesHomeProps) {
    const templates = useSavedTemplates(organisation.id);
    const removal = useDeleteSavedTemplate(organisation.id);
    const [pending, setPending] = useState<SavedTemplateResponse | null>(null);

    const listed = templates.data?.templates;

    function handleConfirm() {
        if (pending === null) {
            return;
        }

        removal.mutate(pending.id, {
            onSettled: () => {
                setPending(null);
            },
        });
    }

    return (
        <OrgPage
            title={PAGE_TITLE}
            description="Templates saved from your boards and your teams' boards. Save a board as a template from its settings."
        >
            {removal.error !== null && <ActionFailure error={removal.error} takeFocus />}
            {templates.error !== null && <LoadFailure message={templates.error.message} onRetry={templates.refetch} />}
            {listed === undefined && templates.error === null && <p className="loading">Loading templates...</p>}
            {listed !== undefined && listed.length === 0 && (
                <p className={styles.empty}>
                    No saved templates yet. Open a board&apos;s settings and choose Save as template to keep its columns
                    for next time.
                </p>
            )}
            {listed !== undefined && listed.length > 0 && (
                <ul className={styles.grid} aria-label="Saved templates">
                    {listed.map(template => (
                        <SavedTemplateCard
                            key={template.id}
                            organisation={organisation}
                            template={template}
                            onDelete={setPending}
                        />
                    ))}
                </ul>
            )}
            <ConfirmDialog
                open={pending !== null}
                title="Delete template?"
                body={
                    <>
                        <bdi>{pending?.name}</bdi> will no longer be offered for new boards. Boards already made from it
                        keep their columns.
                    </>
                }
                confirmLabel="Delete template"
                destructive
                busy={removal.isPending}
                onConfirm={handleConfirm}
                onCancel={() => setPending(null)}
            />
        </OrgPage>
    );
}

export function SavedTemplatesPage() {
    const { current } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    return <SavedTemplatesHome key={current.id} organisation={current} />;
}
