import { type SubmitEvent, useState } from "react";
import { Navigate, useLocation } from "react-router";
import { Button } from "../../../components/Button/Button";
import { ButtonLink } from "../../../components/Button/ButtonLink";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { AuthLayout } from "../../../components/layout/AuthLayout/AuthLayout";
import { TextLink } from "../../../components/TextLink/TextLink";
import { TurnstileWidget } from "../../../components/TurnstileWidget/TurnstileWidget";
import { DISPLAY_NAME_MAX_LENGTH } from "../../../domain/displayName";
import { returnPathFrom } from "../../../domain/returnTo";
import { useRegister } from "../../../hooks/mutations/auth";
import { useAuth } from "../../../hooks/useAuth";
import { usePageTitle } from "../../../hooks/usePageTitle";
import { useProviderSignIn } from "../../../hooks/useProviderSignIn";
import { useReturnPath } from "../../../hooks/useReturnPath";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import { useTurnstile } from "../../../hooks/useTurnstile";
import type { IdentityProviderResponse, RegistrationResponse } from "../../../types/api";
import { errorCode, errorMessage, errorStatus } from "../../../utils/errorMessage";
import { AuthNotice } from "../AuthNotice/AuthNotice";
import { ContinueWithProviders } from "../ContinueWithProviders/ContinueWithProviders";
import { ResendVerification } from "../ResendVerification/ResendVerification";
import styles from "../AuthForm.module.css";

interface RegisterProblems {
    email?: string;
    password?: string;
    check?: string;
}

interface RegisteredProps {
    registration: RegistrationResponse;
}

const TITLE = "Create an account";
const EMAIL_TAKEN = "An account already uses this email address.";
const PASSWORD_TOO_SHORT = "Choose a longer password.";
const SITE_PRIVATE = "This site is private. Ask an admin to invite you.";
const REGISTRATION_CLOSED = "New accounts are closed for now. If you already have an account, sign in.";
const PASSWORD_SIGN_UP_WAITS_FOR_EMAIL =
    "Signing up with an email address and a password is closed until this site can send email.";

function findProblems(email: string, password: string, checkPending: boolean): RegisterProblems {
    const problems: RegisterProblems = {};

    if (email.trim() === "") {
        problems.email = "Enter your email address.";
    }

    if (password === "") {
        problems.password = "Enter a password.";
    }

    if (checkPending) {
        problems.check = "Complete the verification check.";
    }

    return problems;
}

function registerFailure(error: Error): string {
    const code = errorCode(error);

    if (code === "site_private") {
        return SITE_PRIVATE;
    }

    if (code === "registration_closed") {
        return REGISTRATION_CLOSED;
    }

    if (code === "password_too_short") {
        return PASSWORD_TOO_SHORT;
    }

    if (errorStatus(error) === 409) {
        return EMAIL_TAKEN;
    }

    return errorMessage(error, "Creating your account failed. Try again.");
}

function Registered({ registration }: RegisteredProps) {
    if (!registration.verification_required) {
        return (
            <AuthLayout title="Your account is ready">
                <AuthNotice>Sign in to get started.</AuthNotice>
                <ButtonLink to="/login" variant="primary" className={styles.wide}>
                    Sign in
                </ButtonLink>
            </AuthLayout>
        );
    }

    const footer = (
        <p>
            Verified your address? <TextLink to="/login">Sign in</TextLink>
        </p>
    );

    return (
        <AuthLayout title="Check your inbox" footer={footer}>
            <AuthNotice>
                We sent a link to <bdi>{registration.email}</bdi>. Open it to verify your email address, then sign in.
            </AuthNotice>
            <p className={styles.lead}>Nothing arrived? Check your spam folder, or send the link again.</p>
            <ResendVerification email={registration.email} />
        </AuthLayout>
    );
}

export function RegisterPage() {
    usePageTitle(TITLE);

    const { registration_open, site_mode, identity_providers, password_sign_in } = useSiteInfo();
    const { user } = useAuth();
    const returnPath = useReturnPath();
    const location = useLocation();
    const register = useRegister();
    const providerSignIn = useProviderSignIn();
    const turnstile = useTurnstile();
    const [name, setName] = useState("");
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [problems, setProblems] = useState<RegisterProblems>({});

    if (user !== null) {
        return <Navigate to={returnPath} replace />;
    }

    if (register.data !== undefined) {
        return <Registered registration={register.data} />;
    }

    const passwordSignUpOpen = registration_open && password_sign_in;
    const providerSignUpOpen = site_mode === "public" && identity_providers.length > 0;

    if (!passwordSignUpOpen && !providerSignUpOpen) {
        return (
            <AuthLayout title={TITLE}>
                <p className={styles.lead}>{site_mode === "private" ? SITE_PRIVATE : REGISTRATION_CLOSED}</p>
                <ButtonLink to="/login" variant="primary" className={styles.wide}>
                    Sign in
                </ButtonLink>
            </AuthLayout>
        );
    }

    const footer = (
        <p>
            Already have an account? <TextLink to="/login">Sign in</TextLink>
        </p>
    );

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();
        providerSignIn.reset();

        const found = findProblems(email, password, turnstile.enabled && turnstile.token === "");
        setProblems(found);

        if (Object.keys(found).length > 0) {
            register.reset();
            return;
        }

        register.mutate(
            {
                email: email.trim(),
                password,
                display_name: name.trim(),
                turnstile_token: turnstile.enabled ? turnstile.token : undefined,
            },
            { onSettled: turnstile.reset },
        );
    }

    function startProvider(provider: IdentityProviderResponse) {
        setProblems({});
        register.reset();
        providerSignIn.start(provider, {
            purpose: "sign_in",
            return_to: returnPathFrom(location.state) ?? undefined,
        });
    }

    if (!passwordSignUpOpen) {
        return (
            <AuthLayout title={TITLE} footer={footer}>
                <ContinueWithProviders
                    providers={identity_providers}
                    control={providerSignIn}
                    onStart={startProvider}
                    showDivider={false}
                />
                {password_sign_in && <p className={styles.closedNote}>{PASSWORD_SIGN_UP_WAITS_FOR_EMAIL}</p>}
            </AuthLayout>
        );
    }

    function summary(): string | null {
        const listed = Object.values(problems).join(" ");

        if (listed !== "") {
            return listed;
        }

        if (register.error !== null) {
            return registerFailure(register.error);
        }

        return null;
    }

    const message = summary();
    const emailError = problems.email ?? (errorStatus(register.error) === 409 ? EMAIL_TAKEN : undefined);
    const passwordError =
        problems.password ?? (errorCode(register.error) === "password_too_short" ? PASSWORD_TOO_SHORT : undefined);

    return (
        <AuthLayout title={TITLE} footer={footer}>
            {message !== null && <ErrorBanner message={message} takeFocus />}
            {identity_providers.length > 0 && (
                <ContinueWithProviders
                    providers={identity_providers}
                    control={providerSignIn}
                    onStart={startProvider}
                />
            )}
            <form className={styles.form} onSubmit={handleSubmit} noValidate>
                <TextField
                    label="Name (optional)"
                    hint={`Up to ${DISPLAY_NAME_MAX_LENGTH} characters.`}
                    type="text"
                    name="name"
                    autoComplete="name"
                    maxLength={DISPLAY_NAME_MAX_LENGTH}
                    value={name}
                    onChange={event => setName(event.target.value)}
                />
                <TextField
                    label="Email address"
                    type="email"
                    name="email"
                    autoComplete="username"
                    value={email}
                    onChange={event => setEmail(event.target.value)}
                    error={emailError}
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
                <TurnstileWidget control={turnstile} />
                <Button type="submit" variant="primary" className={styles.wide} disabled={register.isPending}>
                    Create account
                </Button>
            </form>
        </AuthLayout>
    );
}
