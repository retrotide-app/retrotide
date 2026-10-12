import { type SubmitEvent, useEffect, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ButtonLink } from "../../../components/Button/ButtonLink";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { AuthLayout } from "../../../components/layout/AuthLayout/AuthLayout";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { TextLink } from "../../../components/TextLink/TextLink";
import { DISPLAY_NAME_MAX_LENGTH } from "../../../domain/displayName";
import { useAcceptInvitation } from "../../../hooks/mutations/auth";
import { useInvitation } from "../../../hooks/queries/auth";
import { usePageTitle } from "../../../hooks/usePageTitle";
import { clearLocationToken, readLocationToken } from "../../../platform/location";
import { errorCode, errorMessage, errorStatus } from "../../../utils/errorMessage";
import { AuthNotice } from "../AuthNotice/AuthNotice";
import styles from "../AuthForm.module.css";

interface AcceptProblems {
    name?: string;
    password?: string;
}

const TITLE = "Create your account";
const EMAIL_TAKEN = "An account already uses this email address.";
const PASSWORD_TOO_SHORT = "Choose a longer password.";
const LINK_REFUSED_STATUS = 400;

function findProblems(name: string, password: string): AcceptProblems {
    const problems: AcceptProblems = {};

    if (name.trim() === "") {
        problems.name = "Enter your name.";
    }

    if (password === "") {
        problems.password = "Enter a password.";
    }

    return problems;
}

function acceptFailure(error: Error): string {
    if (errorCode(error) === "password_too_short") {
        return PASSWORD_TOO_SHORT;
    }

    if (errorStatus(error) === 409) {
        return EMAIL_TAKEN;
    }

    return errorMessage(error, "Creating your account failed. Try again.");
}

export function AcceptInvitationPage() {
    usePageTitle(TITLE);

    const [token] = useState(readLocationToken);
    const invitation = useInvitation(token);
    const accept = useAcceptInvitation();
    const [name, setName] = useState("");
    const [password, setPassword] = useState("");
    const [problems, setProblems] = useState<AcceptProblems>({});

    useEffect(() => {
        clearLocationToken();
    }, []);

    const footer = (
        <p>
            Already have an account? <TextLink to="/login">Sign in</TextLink>
        </p>
    );

    if (token === "") {
        return (
            <AuthLayout title={TITLE} footer={footer}>
                <ErrorBanner message="This link is missing its code. Open the link from your email again." takeFocus />
            </AuthLayout>
        );
    }

    if (accept.isSuccess) {
        return (
            <AuthLayout title="Your account is ready">
                <AuthNotice>Sign in to get started.</AuthNotice>
                <ButtonLink to="/login" variant="primary" className={styles.wide}>
                    Sign in
                </ButtonLink>
            </AuthLayout>
        );
    }

    if (invitation.error !== null) {
        const reason = errorMessage(invitation.error, "Your invitation could not be read.");

        return (
            <AuthLayout title={TITLE} footer={footer}>
                {errorStatus(invitation.error) === LINK_REFUSED_STATUS ? (
                    <ErrorBanner message={reason} takeFocus />
                ) : (
                    <LoadFailure message={reason} onRetry={invitation.refetch} />
                )}
            </AuthLayout>
        );
    }

    if (invitation.data === undefined) {
        return (
            <AuthLayout title={TITLE}>
                <p role="status" className={styles.lead}>
                    Checking your invitation...
                </p>
            </AuthLayout>
        );
    }

    const { email } = invitation.data;

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        const found = findProblems(name, password);
        setProblems(found);

        if (Object.keys(found).length > 0) {
            accept.reset();
            return;
        }

        accept.mutate({ token, display_name: name.trim(), password });
    }

    function summary(): string | null {
        const listed = Object.values(problems).join(" ");

        if (listed !== "") {
            return listed;
        }

        if (accept.error !== null) {
            return acceptFailure(accept.error);
        }

        return null;
    }

    const message = summary();
    const passwordError =
        problems.password ?? (errorCode(accept.error) === "password_too_short" ? PASSWORD_TOO_SHORT : undefined);

    return (
        <AuthLayout title={TITLE} footer={footer}>
            {message !== null && <ErrorBanner message={message} takeFocus />}
            <form className={styles.form} onSubmit={handleSubmit} noValidate>
                <TextField
                    label="Email address"
                    hint="Your invitation was sent to this address."
                    type="email"
                    name="email"
                    autoComplete="username"
                    value={email}
                    readOnly
                />
                <TextField
                    label="Name"
                    hint={`Up to ${DISPLAY_NAME_MAX_LENGTH} characters.`}
                    type="text"
                    name="name"
                    autoComplete="name"
                    maxLength={DISPLAY_NAME_MAX_LENGTH}
                    value={name}
                    onChange={event => setName(event.target.value)}
                    error={problems.name}
                />
                <TextField
                    label="Password"
                    type="password"
                    name="password"
                    autoComplete="new-password"
                    value={password}
                    onChange={event => setPassword(event.target.value)}
                    error={passwordError}
                />
                <Button type="submit" variant="primary" className={styles.wide} disabled={accept.isPending}>
                    Create account
                </Button>
            </form>
        </AuthLayout>
    );
}
