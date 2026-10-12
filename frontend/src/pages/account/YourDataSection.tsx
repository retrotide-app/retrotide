import { ButtonLink } from "../../components/Button/ButtonLink";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import { useSelfExportUrl } from "../../hooks/queries/exports";
import styles from "./AccountSection.module.css";

export function YourDataSection() {
    const exportUrl = useSelfExportUrl();

    return (
        <SettingsRow
            title="Your data"
            headingLevel={2}
            description="Download a copy of the data held about you, including the ways you sign in, as a JSON file."
        >
            <div className={styles.actions}>
                <ButtonLink href={exportUrl} download>
                    Download your data
                </ButtonLink>
            </div>
        </SettingsRow>
    );
}
