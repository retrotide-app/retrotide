import type { ChangeEvent } from "react";
import { useLocation, useNavigate } from "react-router";
import { organisationOptionLabel, switchedOrganisationPath } from "../../../domain/organisations";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import { FormField } from "../../FormField/FormField";
import { LoadFailure } from "../../LoadFailure/LoadFailure";
import { Select } from "../../Select/Select";
import styles from "./OrgSwitcher.module.css";

export function OrgSwitcher() {
    const { organisations, current, error, retry, selectOrganisation } = useCurrentOrg();
    const location = useLocation();
    const navigate = useNavigate();

    if (error !== null) {
        return (
            <div className={styles.switcher}>
                <LoadFailure message={error.message} onRetry={retry} />
            </div>
        );
    }

    if (current === null) {
        return null;
    }

    const handleChange = (event: ChangeEvent<HTMLSelectElement>) => {
        const chosen = organisations.find(organisation => organisation.id === event.target.value);

        if (chosen === undefined) {
            throw new Error(`unknown organisation "${event.target.value}"`);
        }

        selectOrganisation(chosen.id);

        const next = switchedOrganisationPath(location.pathname, chosen.slug);

        if (next !== null) {
            navigate(next);
        }
    };

    return (
        <div className={styles.switcher}>
            <FormField label="Organisation">
                {control => (
                    <Select {...control} size="small" value={current.id} onChange={handleChange}>
                        {organisations.map(organisation => (
                            <option key={organisation.id} value={organisation.id}>
                                {organisationOptionLabel(organisation)}
                            </option>
                        ))}
                    </Select>
                )}
            </FormField>
        </div>
    );
}
