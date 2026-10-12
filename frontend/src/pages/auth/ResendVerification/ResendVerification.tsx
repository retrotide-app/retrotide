import { type SubmitEvent, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { TurnstileWidget } from "../../../components/TurnstileWidget/TurnstileWidget";
import { emailDeliveryRefusal } from "../../../domain/refusals";
import { useResendVerification } from "../../../hooks/mutations/auth";
import { useTurnstile } from "../../../hooks/useTurnstile";
import { errorMessage } from "../../../utils/errorMessage";
import { AuthNotice } from "../AuthNotice/AuthNotice";
import styles from "../AuthForm.module.css";

interface ResendVerificationProps {
    email?: string;
}

interface ResendProblems {
    email?: string;
    check?: string;
}

function findProblems(email: string, checkPending: boolean): ResendProblems {
    const problems: ResendProblems = {};

    if (email === "") {
        problems.email = "Enter your email address.";
    }

    if (checkPending) {
        problems.check = "Complete the verification check.";
    }

    return problems;
}

export function ResendVerification({ email }: ResendVerificationProps) {
    const resend = useResendVerification();
    const turnstile = useTurnstile();
    const [typed, setTyped] = useState("");
    const [problems, setProblems] = useState<ResendProblems>({});

    const known = email !== undefined;
    const address = (email ?? typed).trim();

    if (resend.isSuccess && known) {
        return (
            <AuthNotice>
                Link sent to <bdi>{address}</bdi>. Check your inbox.
            </AuthNotice>
        );
    }

    if (resend.isSuccess) {
        return (
            <AuthNotice>
                Check your inbox. If an account uses <bdi>{address}</bdi> and needs verifying, we sent it a new link.
            </AuthNotice>
        );
    }

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        const found = findProblems(address, turnstile.enabled && turnstile.token === "");
        setProblems(found);

        if (Object.keys(found).length > 0) {
            resend.reset();
            return;
        }

        resend.mutate(
            { email: address, turnstile_token: turnstile.enabled ? turnstile.token : undefined },
            { onSettled: turnstile.reset },
        );
    }

    function summary(): string | null {
        const listed = Object.values(problems).join(" ");

        if (listed !== "") {
            return listed;
        }

        if (resend.error !== null) {
            return (
                emailDeliveryRefusal(resend.error) ??
                errorMessage(resend.error, "The link could not be sent. Try again.")
            );
        }

        return null;
    }

    const message = summary();

    return (
        <form
            className={`${styles.form} ${styles.gapBelow}`}
            onSubmit={handleSubmit}
            aria-label="Get a new verification link"
            noValidate
        >
            {message !== null && <ErrorBanner message={message} takeFocus />}
            {!known && (
                <>
                    <p className={styles.lead}>Enter your email address and we will send you a new link.</p>
                    <TextField
                        label="Email address"
                        type="email"
                        name="email"
                        autoComplete="email"
                        value={typed}
                        onChange={event => setTyped(event.target.value)}
                        error={problems.email}
                    />
                </>
            )}
            <TurnstileWidget control={turnstile} />
            <Button
                type="submit"
                variant={known ? "secondary" : "primary"}
                className={styles.wide}
                disabled={resend.isPending}
            >
                {known ? "Resend link" : "Send new link"}
            </Button>
        </form>
    );
}
