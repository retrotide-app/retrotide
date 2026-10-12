import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import styles from "./LoadFailure.module.css";

interface LoadFailureProps {
    message: string;
    onRetry: () => unknown;
}

export function LoadFailure({ message, onRetry }: LoadFailureProps) {
    return (
        <div className={styles.failure}>
            <ErrorBanner message={message} />
            <div>
                <Button type="button" onClick={() => onRetry()}>
                    Try again
                </Button>
            </div>
        </div>
    );
}
