import type { TurnstileControl } from "../../hooks/useTurnstile";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import styles from "./TurnstileWidget.module.css";

interface TurnstileWidgetProps {
    control: TurnstileControl;
}

export function TurnstileWidget({ control }: TurnstileWidgetProps) {
    const { enabled, error, attach } = control;

    if (!enabled) {
        return null;
    }

    return (
        <div className={styles.widget}>
            <div ref={attach} />
            {error !== null && <ErrorBanner message={error} />}
        </div>
    );
}
