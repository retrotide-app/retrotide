import { useEffect, useRef } from "react";
import { planAllowance } from "../../../domain/entitlements";
import type { UpgradeRequiredResponse } from "../../../types/api";
import styles from "./UpgradePrompt.module.css";

interface UpgradePromptProps {
    upgrade: Pick<UpgradeRequiredResponse, "plan" | "limit" | "allowed">;
    takeFocus?: boolean;
}

export function UpgradePrompt({ upgrade, takeFocus = false }: UpgradePromptProps) {
    const promptRef = useRef<HTMLDivElement>(null);
    const allowance = planAllowance(upgrade);

    useEffect(() => {
        if (!takeFocus) {
            return;
        }

        promptRef.current?.focus();
    }, [takeFocus, allowance]);

    return (
        <div ref={promptRef} role="alert" tabIndex={takeFocus ? -1 : undefined} className={styles.prompt}>
            <p className={styles.allowance}>{allowance}</p>
            <p className={styles.note}>Paid plans are coming. Until then, this limit stays.</p>
        </div>
    );
}
