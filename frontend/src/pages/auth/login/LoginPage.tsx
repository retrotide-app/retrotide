import { type SubmitEvent, useState } from "react";
import { Navigate, useLocation, useNavigate, useSearchParams } from "react-router";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { AuthLayout } from "../../../components/layout/AuthLayout/AuthLayout";
import { TextLink } from "../../../components/TextLink/TextLink";
import { ToggleSwitch } from "../../../components/ToggleSwitch/ToggleSwitch";
import { TurnstileWidget } from "../../../components/TurnstileWidget/TurnstileWidget";
import {
    identityProviderName,
    PROVIDER_CALLBACK_ERROR_PARAM,
    PROVIDER_CALLBACK_PROVIDER_PARAM,
    providerCallbackMessage,
} from "../../../domain/providerCallback";
import { returnPathFrom } from "../../../domain/returnTo";
import { isTwoFactorChallenge, TWO_FACTOR_PATH } from "../../../domain/twoFactor";
import { useLogin } from "../../../hooks/mutations/auth";
import { useAuth } from "../../../hooks/useAuth";
import { usePageTitle } from "../../../hooks/usePageTitle";
import { useProviderSignIn } from "../../../hooks/useProviderSignIn";
import { useReturnPath } from "../../../hooks/useReturnPath";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import { useTurnstile } from "../../../hooks/useTurnstile";
import type { IdentityProviderResponse } from "../../../types/api";
import { errorCode, errorMessage } from "../../../utils/errorMessage";
import { ContinueWithProviders } from "../ContinueWithProviders/ContinueWithProviders";
import { ResendVerification } from "../ResendVerification/ResendVerification";
import styles from "../AuthForm.module.css";

interface LoginProblems {
    email?: string;
    password?: string;
    check?: string;
}

const ACCOUNT_PATH = "/account";

const EMAIL_UNVERIFIED = "email_unverified";

const ACCOUNT_UNVERIFIED = "account_unverified";

const UNVERIFIED = "Verify your email address before you sign in.";

const PASSWORD_SIGN_IN_DISABLED = "password_sign_in_disabled";

const PASSWORD_SIGN_IN_OFF = "Password sign-in is turned off on this site.";

const SIGN_IN_FAILED = "Signing in failed. Try again.";

const SIGN_IN_NOT_KEPT =
    "You signed in, but this browser did not keep you signed in. Allow cookies for this site, then try again.";

function findProblems(email: string, password: string, checkPending: boolean): LoginProblems {
    const problems: LoginProblems = {};

    if (email.trim() === "") {
        problems.email = "Enter your email address.";
    }

    if (password === "") {
        problems.password = "Enter your password.";
    }

    if (checkPending) {
        problems.check = "Complete the verification check.";
    }

    return problems;
}

function callbackFailure(code: string | null, providerName: string | null): string | null {
    if (code === null || code === "") {
        return null;
    }

    if (providerName === null) {
        return SIGN_IN_FAILED;
    }

    return (
        providerCallbackMessage(code, "sign_in", providerName) ??
        providerCallbackMessage("failed", "sign_in", providerName)
    );
}

function accountRefusal(code: string, provider: string | null): string {
    const query = new URLSearchParams({ [PROVIDER_CALLBACK_ERROR_PARAM]: code });

    if (provider !== null && provider !== "") {
        query.set(PROVIDER_CALLBACK_PROVIDER_PARAM, provider);
    }

    return `${ACCOUNT_PATH}?${query.toString()}`;
}

export function LoginPage() {
    usePageTitle("Sign in");

    const { registration_open, site_mode, email_enabled, identity_providers, password_sign_in } = useSiteInfo();
    const { user, error: sessionError } = useAuth();
    const returnPath = useReturnPath();
    const location = useLocation();
    const navigate = useNavigate();
    const [searchParams] = useSearchParams();
    const login = useLogin();
    const providerSignIn = useProviderSignIn();
    const turnstile = useTurnstile();
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [remember, setRemember] = useState(false);
    const [problems, setProblems] = useState<LoginProblems>({});
    const [attempted, setAttempted] = useState(false);

    const callbackCode = searchParams.get(PROVIDER_CALLBACK_ERROR_PARAM);
    const callbackProvider = searchParams.get(PROVIDER_CALLBACK_PROVIDER_PARAM);

    if (user !== null && !attempted && callbackCode !== null && callbackCode !== "") {
        return <Navigate to={accountRefusal(callbackCode, callbackProvider)} replace />;
    }

    if (user !== null) {
        return <Navigate to={returnPath} replace />;
    }

    if (callbackCode === ACCOUNT_UNVERIFIED) {
        const backToSignIn = (
            <p>
                <TextLink to="/login">Back to sign in</TextLink>
            </p>
        );

        return (
            <AuthLayout title="Sign in" footer={backToSignIn}>
                <ErrorBanner message={UNVERIFIED} takeFocus />
                <ResendVerification />
            </AuthLayout>
        );
    }

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();
        setAttempted(true);
        providerSignIn.reset();

        const found = findProblems(email, password, turnstile.enabled && turnstile.token === "");
        setProblems(found);

        if (Object.keys(found).length > 0) {
            login.reset();
            return;
        }

        login.mutate(
            {
                email: email.trim(),
                password,
                remember,
                turnstile_token: turnstile.enabled ? turnstile.token : undefined,
            },
            {
                onSuccess: response => {
                    if (isTwoFactorChallenge(response)) {
                        navigate(TWO_FACTOR_PATH, { state: location.state });
                    }
                },
                onSettled: turnstile.reset,
            },
        );
    }

    function startProvider(provider: IdentityProviderResponse) {
        setAttempted(true);
        setProblems({});
        login.reset();
        providerSignIn.start(provider, {
            purpose: "sign_in",
            return_to: returnPathFrom(location.state) ?? undefined,
        });
    }

    function summary(): string | null {
        const listed = Object.values(problems).join(" ");

        if (listed !== "") {
            return listed;
        }

        if (errorCode(login.error) === EMAIL_UNVERIFIED) {
            return UNVERIFIED;
        }

        if (errorCode(login.error) === PASSWORD_SIGN_IN_DISABLED) {
            return PASSWORD_SIGN_IN_OFF;
        }

        if (login.error !== null) {
            return errorMessage(login.error, SIGN_IN_FAILED);
        }

        if (login.isSuccess && !isTwoFactorChallenge(login.data)) {
            return sessionError?.message ?? SIGN_IN_NOT_KEPT;
        }

        if (attempted) {
            return null;
        }

        return callbackFailure(callbackCode, identityProviderName(identity_providers, callbackProvider));
    }

    const message = summary();
    const unverifiedEmail = errorCode(login.error) === EMAIL_UNVERIFIED ? login.variables?.email : undefined;

    const providerSignUpOpen = site_mode === "public" && identity_providers.length > 0;
    const signUpWaitsForEmail = !registration_open && site_mode === "public" && !email_enabled;

    const footer =
        registration_open || providerSignUpOpen ? (
            <p>
                New here? <TextLink to="/register">Create an account</TextLink>
            </p>
        ) : signUpWaitsForEmail ? (
            <>
                <p>
                    New here?{" "}
                    <span className={styles.disabledLink} aria-disabled="true">
                        Create an account
                    </span>
                </p>
                <p className={styles.closedNote}>New accounts are closed until this site can send email.</p>
            </>
        ) : undefined;

    if (!password_sign_in) {
        return (
            <AuthLayout title="Sign in" footer={footer}>
                {message !== null && <ErrorBanner message={message} takeFocus />}
                <ContinueWithProviders
                    providers={identity_providers}
                    control={providerSignIn}
                    onStart={startProvider}
                    showDivider={false}
                />
            </AuthLayout>
        );
    }

    return (
        <AuthLayout title="Sign in" footer={footer}>
            {message !== null && <ErrorBanner message={message} takeFocus />}
            {unverifiedEmail !== undefined && <ResendVerification key={unverifiedEmail} email={unverifiedEmail} />}
            {identity_providers.length > 0 && (
                <ContinueWithProviders
                    providers={identity_providers}
                    control={providerSignIn}
                    onStart={startProvider}
                />
            )}
            <form className={styles.form} onSubmit={handleSubmit} noValidate>
                <TextField
                    label="Email address"
                    type="email"
                    name="email"
                    autoComplete="username"
                    value={email}
                    onChange={event => setEmail(event.target.value)}
                    error={problems.email}
                />
                <TextField
                    label="Password"
                    type="password"
                    name="password"
                    autoComplete="current-password"
                    value={password}
                    onChange={event => setPassword(event.target.value)}
                    error={problems.password}
                />
                <ToggleSwitch
                    label="Remember me"
                    description="Stay signed in on this device after you close the browser."
                    enabled={remember}
                    onChange={setRemember}
                />
                <TurnstileWidget control={turnstile} />
                <Button type="submit" variant="primary" className={styles.wide} disabled={login.isPending}>
                    Sign in
                </Button>
            </form>
            <p className={styles.links}>
                <TextLink to="/forgot-password">Forgotten your password?</TextLink>
            </p>
        </AuthLayout>
    );
}
