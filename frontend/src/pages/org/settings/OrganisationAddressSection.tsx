import { useState } from "react";
import { Banner } from "../../../components/Banner/Banner";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../../components/FormDialog/FormDialog";
import { TextField } from "../../../components/FormField/TextField";
import { SettingsRow } from "../../../components/Settings/SettingsRow";
import { ADDRESS_REFUSALS, fieldRefusal } from "../../../domain/refusals";
import { useUpdateOrganisation } from "../../../hooks/mutations/organisations";
import type { OrganisationResponse } from "../../../types/api";
import { ActionFailure } from "../shared/ActionFailure";

interface OrganisationAddressSectionProps {
    organisation: OrganisationResponse;
}

const ADDRESS_MISSING = "Enter an address.";

export function OrganisationAddressSection({ organisation }: OrganisationAddressSectionProps) {
    const update = useUpdateOrganisation();
    const [open, setOpen] = useState(false);
    const [slug, setSlug] = useState(organisation.slug);
    const [problem, setProblem] = useState<string | null>(null);

    const fieldError = problem ?? fieldRefusal(update.error, ADDRESS_REFUSALS);

    function handleSubmit() {
        if (slug.trim() === "") {
            update.reset();
            setProblem(ADDRESS_MISSING);
            return;
        }

        setProblem(null);
        update.mutate(
            {
                orgId: organisation.id,
                name: organisation.name,
                slug,
                allow_guests: organisation.allow_guests,
            },
            { onSuccess: () => setOpen(false) },
        );
    }

    const description = (
        <>
            Links to this organisation use the address <bdi>{organisation.slug}</bdi>.
        </>
    );

    return (
        <SettingsRow title="Address" headingLevel={2} description={description}>
            <Button
                type="button"
                onClick={() => {
                    update.reset();
                    setProblem(null);
                    setSlug(organisation.slug);
                    setOpen(true);
                }}
            >
                Change address
            </Button>
            {update.isSuccess && !open && (
                <Banner tone="success" role="status">
                    Address saved.
                </Banner>
            )}
            <FormDialog
                open={open}
                title="Change address"
                submitLabel="Save address"
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
                    label="Organisation address"
                    hint="3 to 40 lower-case letters, digits or hyphens. Links that use the old address stop working."
                    error={fieldError}
                    value={slug}
                    onChange={e => setSlug(e.target.value)}
                    autoComplete="off"
                    autoCapitalize="none"
                    spellCheck={false}
                    required
                />
            </FormDialog>
        </SettingsRow>
    );
}
