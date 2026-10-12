import { UpgradePrompt } from "../../components/billing/UpgradePrompt/UpgradePrompt";
import type { UpgradeRequiredResponse } from "../../types/api";
import styles from "./InviteAcceptancePage.module.css";

interface InviteUpgradeProps {
    upgrade: UpgradeRequiredResponse;
    orgName: string;
}

export function InviteUpgrade({ upgrade, orgName }: InviteUpgradeProps) {
    return (
        <div className={styles.upgrade}>
            <UpgradePrompt upgrade={upgrade} takeFocus />
            <p>
                <bdi>{orgName}</bdi> has reached its plan&apos;s limit. Ask the person who invited you.
            </p>
        </div>
    );
}
