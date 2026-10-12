import { useState } from "react";
import { Banner } from "../../../components/Banner/Banner";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../../components/FormDialog/FormDialog";
import { TextField } from "../../../components/FormField/TextField";
import { SettingsRow } from "../../../components/Settings/SettingsRow";
import { ORGANISATION_NAME_MAX_LENGTH } from "../../../domain/organisations";
import { fieldRefusal, ORGANISATION_NAME_REFUSALS } from "../../../domain/refusals";
import { useUpdateOrganisation } from "../../../hooks/mutations/organisations";
import type { OrganisationResponse } from "../../../types/api";
import { ActionFailure } from "../shared/ActionFailure";

interface OrganisationNameSectionProps {
    organisation: OrganisationResponse;
}

const NAME_MISSING = "Enter an organisation name.";

export function OrganisationNameSection({ organisation }: OrganisationNameSectionProps) {
    const update = useUpdateOrganisation();
    const [open, setOpen] = useState(false);
    const [name, setName] = useState(organisation.name);
    const [problem, setProblem] = useState<string | null>(null);

    const fieldError = problem ?? fieldRefusal(update.error, ORGANISATION_NAME_REFUSALS);

    function handleSubmit() {
        if (name.trim() === "") {
            update.reset();
            setProblem(NAME_MISSING);
            return;
        }

        setProblem(null);
        update.mutate(
            {
                orgId: organisation.id,
                name,
                slug: organisation.slug,
                allow_guests: organisation.allow_guests,
            },
            { onSuccess: () => setOpen(false) },
        );
    }

    const description = (
        <>
            Your organisation is called <bdi>{organisation.name}</bdi>. Members see it, and so do invitations and
            emails.
        </>
    );

    return (
        <SettingsRow title="Name" headingLevel={2} description={description}>
            <Button
                type="button"
                onClick={() => {
                    update.reset();
                    setProblem(null);
                    setName(organisation.name);
                    setOpen(true);
                }}
            >
                Rename
            </Button>
            {update.isSuccess && !open && (
                <Banner tone="success" role="status">
                    Name saved.
                </Banner>
            )}
            <FormDialog
                open={open}
                title="Rename organisation"
                submitLabel="Save name"
                busy={update.isPending}
                onSubmit={handleSubmit}
                onCancel={() => {
                    setOpen(false);
                    setProblem(null);
                    update.reset();
                }}
            >
                {problem !== null && <ErrorBanner message={problem} takeFocus />}
                {problem === null && update.error !== null && <ActionFailure error={update.error} takeFocus />}
                <TextField
                    label="Organisation name"
                    hint={`Up to ${ORGANISATION_NAME_MAX_LENGTH} characters.`}
                    error={fieldError}
                    value={name}
                    onChange={e => setName(e.target.value)}
                    maxLength={ORGANISATION_NAME_MAX_LENGTH}
                    autoComplete="organization"
                    required
                />
            </FormDialog>
        </SettingsRow>
    );
}
