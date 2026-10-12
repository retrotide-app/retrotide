import { type SubmitEvent, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { AuthLayout } from "../../../components/layout/AuthLayout/AuthLayout";
import { TextLink } from "../../../components/TextLink/TextLink";
import { TurnstileWidget } from "../../../components/TurnstileWidget/TurnstileWidget";
import { emailDeliveryRefusal } from "../../../domain/refusals";
import { useForgotPassword } from "../../../hooks/mutations/auth";
import { usePageTitle } from "../../../hooks/usePageTitle";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import { useTurnstile } from "../../../hooks/useTurnstile";
import { errorMessage } from "../../../utils/errorMessage";
import { AuthNotice } from "../AuthNotice/AuthNotice";
import styles from "../AuthForm.module.css";

interface ForgotProblems {
    email?: string;
    check?: string;
}

const TITLE = "Reset your password";
const PASSWORD_SIGN_IN_OFF = "Password sign-in is turned off on this site.";

function findProblems(email: string, checkPending: boolean): ForgotProblems {
    const problems: ForgotProblems = {};

    if (email.trim() === "") {
        problems.email = "Enter your email address.";
    }

    if (checkPending) {
        problems.check = "Complete the verification check.";
    }

    return problems;
}

export function ForgotPasswordPage() {
    usePageTitle(TITLE);

    const { password_sign_in } = useSiteInfo();
    const forgot = useForgotPassword();
    const turnstile = useTurnstile();
    const [email, setEmail] = useState("");
    const [problems, setProblems] = useState<ForgotProblems>({});

    const footer = (
        <p>
            <TextLink to="/login">Back to sign in</TextLink>
        </p>
    );

    if (!password_sign_in) {
        return (
            <AuthLayout title={TITLE} footer={footer}>
                <AuthNotice>{PASSWORD_SIGN_IN_OFF}</AuthNotice>
            </AuthLayout>
        );
    }

    if (forgot.isSuccess) {
        return (
            <AuthLayout title={TITLE} footer={footer}>
                <AuthNotice>
                    Check your inbox. If an account uses <bdi>{email.trim()}</bdi>, we sent it a link to reset the
                    password. The link expires after one hour.
                </AuthNotice>
            </AuthLayout>
        );
    }

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        const found = findProblems(email, turnstile.enabled && turnstile.token === "");
        setProblems(found);

        if (Object.keys(found).length > 0) {
            forgot.reset();
            return;
        }

        forgot.mutate(
            { email: email.trim(), turnstile_token: turnstile.enabled ? turnstile.token : undefined },
            { onSettled: turnstile.reset },
        );
    }

    function summary(): string | null {
        const listed = Object.values(problems).join(" ");

        if (listed !== "") {
            return listed;
        }

        if (forgot.error !== null) {
            return (
                emailDeliveryRefusal(forgot.error) ??
                errorMessage(forgot.error, "The link could not be sent. Try again.")
            );
        }

        return null;
    }

    const message = summary();

    return (
        <AuthLayout title={TITLE} footer={footer}>
            {message !== null && <ErrorBanner message={message} takeFocus />}
            <p className={styles.lead}>
                Enter the email address you sign in with. We will send you a link to choose a new password.
            </p>
            <form className={styles.form} onSubmit={handleSubmit} noValidate>
                <TextField
                    label="Email address"
                    type="email"
                    name="email"
                    autoComplete="email"
                    value={email}
                    onChange={event => setEmail(event.target.value)}
                    error={problems.email}
                />
                <TurnstileWidget control={turnstile} />
                <Button type="submit" variant="primary" className={styles.wide} disabled={forgot.isPending}>
                    Send reset link
                </Button>
            </form>
        </AuthLayout>
    );
}
