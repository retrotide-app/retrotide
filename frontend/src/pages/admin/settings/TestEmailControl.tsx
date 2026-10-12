import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { Toast } from "../../../components/Toast/Toast";
import { useSendTestEmail } from "../../../hooks/mutations/admin";
import { useAuth } from "../../../hooks/useAuth";
import styles from "./AdminSettingsPage.module.css";

interface TestEmailControlProps {
    unsaved: boolean;
    saving: boolean;
    saveThen: (next: () => void) => void;
}

export function TestEmailControl({ unsaved, saving, saveThen }: TestEmailControlProps) {
    const { user } = useAuth();
    const test = useSendTestEmail();

    function handleClick() {
        test.reset();

        if (unsaved) {
            saveThen(() => test.mutate());
            return;
        }

        test.mutate();
    }

    return (
        <div className={styles.testEmail}>
            <p className={styles.note}>
                Sends a test email to {user === null ? "your address" : <bdi>{user.email}</bdi>}.
                {unsaved && " Your changes are saved first."}
            </p>
            {test.error !== null && <ErrorBanner message={test.error.message} takeFocus />}
            <div>
                <Button type="button" onClick={handleClick} disabled={test.isPending || saving}>
                    {unsaved ? "Save and send test email" : "Send test email"}
                </Button>
            </div>
            {test.isSuccess && (
                <Toast variant="success" onDismiss={test.reset}>
                    Test email sent. Check your inbox.
                </Toast>
            )}
        </div>
    );
}
