import { Banner } from "../../../components/Banner/Banner";
import { SettingsRow } from "../../../components/Settings/SettingsRow";
import { ToggleSwitch } from "../../../components/ToggleSwitch/ToggleSwitch";
import { useUpdateOrganisation } from "../../../hooks/mutations/organisations";
import type { OrganisationResponse } from "../../../types/api";
import { ActionFailure } from "../shared/ActionFailure";

interface GuestAccessSectionProps {
    organisation: OrganisationResponse;
}

export function GuestAccessSection({ organisation }: GuestAccessSectionProps) {
    const update = useUpdateOrganisation();
    const allowed = update.variables?.allow_guests;

    return (
        <SettingsRow
            title="Guests"
            headingLevel={2}
            description="Let people without an account join boards through a share link."
        >
            {update.error !== null && <ActionFailure error={update.error} takeFocus />}
            <ToggleSwitch
                label="Allow guests"
                enabled={organisation.allow_guests}
                disabled={update.isPending}
                onChange={next =>
                    update.mutate({
                        orgId: organisation.id,
                        name: organisation.name,
                        slug: organisation.slug,
                        allow_guests: next,
                    })
                }
            />
            {update.isSuccess && (
                <Banner tone="success" role="status">
                    {allowed === true ? "Guests are now allowed." : "Guests are no longer allowed."}
                </Banner>
            )}
        </SettingsRow>
    );
}
