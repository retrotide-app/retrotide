import { type SubmitEvent, useState } from "react";
import { Button } from "../../components/Button/Button";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../components/FormField/TextField";
import { AuthLayout } from "../../components/layout/AuthLayout/AuthLayout";
import { TextLink } from "../../components/TextLink/TextLink";
import { DISPLAY_NAME_MAX_LENGTH } from "../../domain/displayName";
import { readUpgradeRequired } from "../../domain/entitlements";
import { INVITE_PATH, joinedOrganisationPath } from "../../domain/organisations";
import {
    DISPLAY_NAME_REFUSALS,
    EMAIL_DELIVERY_REFUSALS,
    EMAIL_REFUSALS,
    fieldRefusal,
    refusedWith,
} from "../../domain/refusals";
import { returnState } from "../../domain/returnTo";
import { useRegisterThroughInvite } from "../../hooks/mutations/invites";
import { useProviderSignIn } from "../../hooks/useProviderSignIn";
import { useSiteInfo } from "../../hooks/useSiteInfo";
import type { IdentityProviderResponse, InviteDetailsResponse, InviteRegistrationResponse } from "../../types/api";
import { errorCode, errorMessage } from "../../utils/errorMessage";
import { AuthNotice } from "../auth/AuthNotice/AuthNotice";
import authStyles from "../auth/AuthForm.module.css";
import { ContinueWithProviders } from "../auth/ContinueWithProviders/ContinueWithProviders";
import { ResendVerification } from "../auth/ResendVerification/ResendVerification";
import { InviteDetails } from "./InviteDetails";
import { ACCEPT_TITLE, EMAIL_MISMATCH_CODE, INVITE_INVALID, INVITE_INVALID_CODE } from "./inviteMessages";
import { InviteUpgrade } from "./InviteUpgrade";
import styles from "./InviteAcceptancePage.module.css";

interface InviteSignUpProps {
    invite: InviteDetailsResponse;
    token: string;
    forget: () => void;
}

interface RegisteredProps {
    registration: InviteRegistrationResponse;
}

interface SignUpProblems {
    name?: string;
    email?: string;
    password?: string;
}

const LOGIN_PATH = "/login";

const PASSWORD_TOO_SHORT_CODE = "password_too_short";

const EMAIL_TAKEN = "An account already uses this email address. Sign in to accept the invite.";

const PASSWORD_TOO_SHORT = "Choose a longer password.";

const EMAIL_MISMATCH = "This invite is for a different email address.";

const REGISTER_FAILED = "Creating your account failed. Try again.";

const NEW_HERE = "New here? Create an account to join.";

const NEW_HERE_PROVIDERS_ONLY = "New here? Continue with a sign-in provider to create an account and join.";

const CANNOT_VERIFY =
    "This site cannot send email yet, so it cannot confirm a new address. Ask whoever invited you to send the invite to your email address instead.";

function findProblems(name: string, email: string, password: string): SignUpProblems {
    const problems: SignUpProblems = {};

    if (name.trim() === "") {
        problems.name = "Enter your name.";
    }

    if (email.trim() === "") {
        problems.email = "Enter your email address.";
    }

    if (password === "") {
        problems.password = "Enter a password.";
    }

    return problems;
}

function registerFailure(error: Error): string {
    const code = errorCode(error);

    if (code === INVITE_INVALID_CODE) {
        return INVITE_INVALID;
    }

    if (code === PASSWORD_TOO_SHORT_CODE) {
        return PASSWORD_TOO_SHORT;
    }

    if (code === EMAIL_MISMATCH_CODE) {
        return EMAIL_MISMATCH;
    }

    if (refusedWith(error, EMAIL_REFUSALS)) {
        return EMAIL_TAKEN;
    }

    if (refusedWith(error, EMAIL_DELIVERY_REFUSALS)) {
        return CANNOT_VERIFY;
    }

    return errorMessage(error, REGISTER_FAILED);
}

function Registered({ registration }: RegisteredProps) {
    const { joined } = registration;
    const signedIn = returnState(joinedOrganisationPath(joined), "");

    if (!registration.verification_required) {
        return (
            <AuthLayout title="Your account is ready">
                <AuthNotice>
                    You joined <bdi>{joined.org_name}</bdi>. Sign in to get started.
                </AuthNotice>
                <ButtonLink to={LOGIN_PATH} state={signedIn} variant="primary" className={authStyles.wide}>
                    Sign in
                </ButtonLink>
            </AuthLayout>
        );
    }

    const footer = (
        <p>
            Verified your address?{" "}
            <TextLink to={LOGIN_PATH} state={signedIn}>
                Sign in
            </TextLink>
        </p>
    );

    return (
        <AuthLayout title="Check your inbox" footer={footer}>
            <AuthNotice>
                You joined <bdi>{joined.org_name}</bdi>. We sent a link to <bdi>{registration.email}</bdi>. Open it to
                verify your email address, then sign in.
            </AuthNotice>
            <p className={authStyles.lead}>Nothing arrived? Check your spam folder, or send the link again.</p>
            <ResendVerification email={registration.email} />
        </AuthLayout>
    );
}

export function InviteSignUp({ invite, token, forget }: InviteSignUpProps) {
    const { identity_providers, password_sign_in } = useSiteInfo();
    const register = useRegisterThroughInvite();
    const providerSignIn = useProviderSignIn();
    const [name, setName] = useState("");
    const [typedEmail, setTypedEmail] = useState("");
    const [password, setPassword] = useState("");
    const [problems, setProblems] = useState<SignUpProblems>({});

    if (register.data !== undefined) {
        return <Registered registration={register.data} />;
    }

    const boundAddress = invite.email ?? "";
    const email = boundAddress === "" ? typedEmail : boundAddress;

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();
        providerSignIn.reset();

        const found = findProblems(name, email, password);
        setProblems(found);

        if (Object.keys(found).length > 0) {
            register.reset();
            return;
        }

        register.mutate(
            { token, email: email.trim(), display_name: name.trim(), password },
            {
                onSuccess: forget,
                onError: failure => {
                    if (errorCode(failure) === INVITE_INVALID_CODE) {
                        forget();
                    }
                },
            },
        );
    }

    function startProvider(provider: IdentityProviderResponse) {
        setProblems({});
        register.reset();
        providerSignIn.start(provider, { purpose: "sign_in", return_to: INVITE_PATH });
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

    const upgrade = readUpgradeRequired(register.error);
    const message = upgrade === null ? summary() : null;
    const nameError = problems.name ?? fieldRefusal(register.error, DISPLAY_NAME_REFUSALS);
    const emailError = problems.email ?? (refusedWith(register.error, EMAIL_REFUSALS) ? EMAIL_TAKEN : undefined);
    const passwordError =
        problems.password ?? (errorCode(register.error) === PASSWORD_TOO_SHORT_CODE ? PASSWORD_TOO_SHORT : undefined);

    return (
        <AuthLayout title={ACCEPT_TITLE}>
            <InviteDetails invite={invite} />
            {upgrade !== null && <InviteUpgrade upgrade={upgrade} orgName={invite.org_name} />}
            {message !== null && <ErrorBanner message={message} takeFocus />}
            <div className={styles.signIn}>
                <p>Already have an account?</p>
                <ButtonLink to={LOGIN_PATH} state={returnState(INVITE_PATH, "")} className={authStyles.wide}>
                    Sign in
                </ButtonLink>
            </div>
            <p className={authStyles.lead}>{password_sign_in ? NEW_HERE : NEW_HERE_PROVIDERS_ONLY}</p>
            {identity_providers.length > 0 && (
                <ContinueWithProviders
                    providers={identity_providers}
                    control={providerSignIn}
                    onStart={startProvider}
                    showDivider={password_sign_in}
                />
            )}
            {password_sign_in && (
                <form className={authStyles.form} onSubmit={handleSubmit} aria-label="Create an account" noValidate>
                    <TextField
                        label="Name"
                        hint={`Up to ${DISPLAY_NAME_MAX_LENGTH} characters.`}
                        type="text"
                        name="name"
                        autoComplete="name"
                        maxLength={DISPLAY_NAME_MAX_LENGTH}
                        value={name}
                        onChange={event => setName(event.target.value)}
                        error={nameError}
                    />
                    {boundAddress === "" ? (
                        <TextField
                            label="Email address"
                            type="email"
                            name="email"
                            autoComplete="username"
                            value={typedEmail}
                            onChange={event => setTypedEmail(event.target.value)}
                            error={emailError}
                        />
                    ) : (
                        <TextField
                            label="Email address"
                            hint="Your invite was sent to this address."
                            type="email"
                            name="email"
                            autoComplete="username"
                            value={boundAddress}
                            readOnly
                            error={emailError}
                        />
                    )}
                    <TextField
                        label="Password"
                        type="password"
                        name="password"
                        autoComplete="new-password"
                        value={password}
                        onChange={event => setPassword(event.target.value)}
                        error={passwordError}
                    />
                    <Button type="submit" variant="primary" className={authStyles.wide} disabled={register.isPending}>
                        Create account
                    </Button>
                </form>
            )}
        </AuthLayout>
    );
}
