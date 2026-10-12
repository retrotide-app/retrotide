import { type SubmitEvent, useState } from "react";
import { useForgotPassword } from "../../hooks/mutations/auth";
import { useTurnstile } from "../../hooks/useTurnstile";
import { errorMessage } from "../../utils/errorMessage";
import { Banner } from "../Banner/Banner";
import { Button } from "../Button/Button";
import type { ButtonVariant } from "../Button/buttonClassName";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import { TurnstileWidget } from "../TurnstileWidget/TurnstileWidget";
import styles from "./SetPasswordForm.module.css";

interface SetPasswordFormProps {
    email: string;
    variant?: ButtonVariant;
}

const CHECK_PENDING = "Complete the verification check.";

const SEND_FAILED = "The link could not be sent. Try again.";

export function SetPasswordForm({ email, variant = "primary" }: SetPasswordFormProps) {
    const request = useForgotPassword();
    const turnstile = useTurnstile();
    const [checkPending, setCheckPending] = useState(false);

    const failure = request.error === null ? null : errorMessage(request.error, SEND_FAILED);
    const summary = checkPending ? CHECK_PENDING : failure;

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        if (turnstile.enabled && turnstile.token === "") {
            request.reset();
            setCheckPending(true);
            return;
        }

        setCheckPending(false);
        request.mutate(turnstile.enabled ? { email, turnstile_token: turnstile.token } : { email }, {
            onSettled: turnstile.reset,
        });
    }

    return (
        <form className={styles.form} onSubmit={handleSubmit} noValidate>
            {summary !== null && <ErrorBanner message={summary} takeFocus />}
            <TurnstileWidget control={turnstile} />
            <div className={styles.actions}>
                <Button type="submit" variant={variant} disabled={request.isPending}>
                    Set a password
                </Button>
            </div>
            {request.isSuccess && (
                <Banner tone="success" role="status">
                    Check <bdi>{email}</bdi> for the link. It works for one hour, and using it signs you out on every
                    device.
                </Banner>
            )}
        </form>
    );
}
