import { type SubmitEvent, useEffect, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ButtonLink } from "../../../components/Button/ButtonLink";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { AuthLayout } from "../../../components/layout/AuthLayout/AuthLayout";
import { TextLink } from "../../../components/TextLink/TextLink";
import { useResetPassword } from "../../../hooks/mutations/auth";
import { usePageTitle } from "../../../hooks/usePageTitle";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import { clearLocationToken, readLocationToken } from "../../../platform/location";
import { errorCode, errorMessage } from "../../../utils/errorMessage";
import { AuthNotice } from "../AuthNotice/AuthNotice";
import styles from "../AuthForm.module.css";

interface ResetProblems {
    password?: string;
    confirmation?: string;
}

const TITLE = "Choose a new password";
const PASSWORD_TOO_SHORT = "Choose a longer password.";
const PASSWORD_SIGN_IN_OFF = "Password sign-in is turned off on this site.";

function findProblems(password: string, confirmation: string): ResetProblems {
    const problems: ResetProblems = {};

    if (password === "") {
        problems.password = "Enter a new password.";
    }

    if (confirmation === "") {
        problems.confirmation = "Enter the new password again.";
    }

    if (password !== "" && confirmation !== "" && password !== confirmation) {
        problems.confirmation = "The passwords do not match.";
    }

    return problems;
}

function resetFailure(error: Error): string {
    if (errorCode(error) === "password_too_short") {
        return PASSWORD_TOO_SHORT;
    }

    return errorMessage(error, "Your password could not be reset. Try again.");
}

export function ResetPasswordPage() {
    usePageTitle(TITLE);

    const { password_sign_in } = useSiteInfo();
    const [token] = useState(readLocationToken);
    const reset = useResetPassword();
    const [password, setPassword] = useState("");
    const [confirmation, setConfirmation] = useState("");
    const [problems, setProblems] = useState<ResetProblems>({});

    useEffect(() => {
        clearLocationToken();
    }, []);

    if (!password_sign_in) {
        const footer = (
            <p>
                <TextLink to="/login">Back to sign in</TextLink>
            </p>
        );

        return (
            <AuthLayout title={TITLE} footer={footer}>
                <AuthNotice>{PASSWORD_SIGN_IN_OFF}</AuthNotice>
            </AuthLayout>
        );
    }

    if (token === "") {
        return (
            <AuthLayout title={TITLE}>
                <ErrorBanner message="This link is missing its code. Open the link from your email again." takeFocus />
                <div className={styles.actions}>
                    <ButtonLink to="/forgot-password" variant="primary" className={styles.wide}>
                        Get a new reset link
                    </ButtonLink>
                </div>
            </AuthLayout>
        );
    }

    if (reset.isSuccess) {
        return (
            <AuthLayout title={TITLE}>
                <AuthNotice>Your password has been reset, and you are signed out on every device.</AuthNotice>
                <ButtonLink to="/login" variant="primary" className={styles.wide}>
                    Sign in
                </ButtonLink>
            </AuthLayout>
        );
    }

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        const found = findProblems(password, confirmation);
        setProblems(found);

        if (Object.keys(found).length > 0) {
            reset.reset();
            return;
        }

        reset.mutate({ token, new_password: password });
    }

    function summary(): string | null {
        const listed = Object.values(problems).join(" ");

        if (listed !== "") {
            return listed;
        }

        if (reset.error !== null) {
            return resetFailure(reset.error);
        }

        return null;
    }

    const message = summary();
    const passwordError =
        problems.password ?? (errorCode(reset.error) === "password_too_short" ? PASSWORD_TOO_SHORT : undefined);

    const footer = (
        <p>
            Link expired? <TextLink to="/forgot-password">Get a new reset link</TextLink>
        </p>
    );

    return (
        <AuthLayout title={TITLE} footer={footer}>
            {message !== null && <ErrorBanner message={message} takeFocus />}
            <form className={styles.form} onSubmit={handleSubmit} noValidate>
                <TextField
                    label="New password"
                    type="password"
                    name="new-password"
                    autoComplete="new-password"
                    value={password}
                    onChange={event => setPassword(event.target.value)}
                    error={passwordError}
                />
                <TextField
                    label="Confirm new password"
                    type="password"
                    name="confirm-password"
                    autoComplete="new-password"
                    value={confirmation}
                    onChange={event => setConfirmation(event.target.value)}
                    error={problems.confirmation}
                />
                <Button type="submit" variant="primary" className={styles.wide} disabled={reset.isPending}>
                    Reset password
                </Button>
            </form>
        </AuthLayout>
    );
}
