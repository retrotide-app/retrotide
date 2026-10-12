import { useState } from "react";
import { useSearchParams } from "react-router";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../components/FormDialog/FormDialog";
import { FormField } from "../../components/FormField/FormField";
import { TextField } from "../../components/FormField/TextField";
import { Select } from "../../components/Select/Select";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import {
    identityProviderName,
    PROVIDER_CALLBACK_ERROR_PARAM,
    PROVIDER_CALLBACK_PROVIDER_PARAM,
    providerCallbackMessage,
} from "../../domain/providerCallback";
import { providerStartFailure } from "../../domain/providerStart";
import { useIdentities } from "../../hooks/queries/identity";
import { type ProviderSignIn, useProviderSignIn } from "../../hooks/useProviderSignIn";
import { useSiteInfo } from "../../hooks/useSiteInfo";
import type { IdentityProviderResponse } from "../../types/api";
import type { SessionUser } from "../../types/app";
import { errorCode } from "../../utils/errorMessage";
import styles from "./AccountSection.module.css";
import { IdentityList } from "./IdentityList";

interface SignInMethodsSectionProps {
    user: SessionUser;
}

interface LinkProviderProps {
    providers: readonly IdentityProviderResponse[];
    held: readonly IdentityProviderResponse[];
    hasPassword: boolean;
    onOpen: () => void;
}

interface LinkProviderFormProps {
    provider: IdentityProviderResponse;
    control: ProviderSignIn;
    onClose: () => void;
}

interface ConfirmWithProviderFormProps {
    provider: IdentityProviderResponse;
    held: readonly IdentityProviderResponse[];
    control: ProviderSignIn;
    onClose: () => void;
}

const ACCOUNT_PATH = "/account";

const PASSWORD_MISSING = "Enter your current password.";

function LinkProviderForm({ provider, control, onClose }: LinkProviderFormProps) {
    const [password, setPassword] = useState("");
    const [problem, setProblem] = useState<string | null>(null);

    const failure = control.error === null ? null : providerStartFailure(control.error, "link", provider.name);
    const passwordError = problem ?? (errorCode(control.error) === "incorrect_password" ? failure : null);
    const summary = problem ?? failure;

    function handleSubmit() {
        if (password === "") {
            control.reset();
            setProblem(PASSWORD_MISSING);
            return;
        }

        setProblem(null);
        control.start(provider, { purpose: "link", current_password: password, return_to: ACCOUNT_PATH });
    }

    return (
        <FormDialog
            open
            title={`Link ${provider.name} account`}
            submitLabel={`Continue to ${provider.name}`}
            busy={control.pending}
            onSubmit={handleSubmit}
            onCancel={() => {
                control.reset();
                onClose();
            }}
        >
            {summary !== null && <ErrorBanner message={summary} takeFocus />}
            <TextField
                label="Current password"
                hint={`Enter your password to link your ${provider.name} account.`}
                type="password"
                autoComplete="current-password"
                error={passwordError}
                value={password}
                onChange={e => setPassword(e.target.value)}
                required
            />
        </FormDialog>
    );
}

function ConfirmWithProviderForm({ provider, held, control, onClose }: ConfirmWithProviderFormProps) {
    const [chosenId, setChosenId] = useState(held[0].id);

    const chosen = held.find(candidate => candidate.id === chosenId) ?? held[0];
    const failure = control.error === null ? null : providerStartFailure(control.error, "link", provider.name);

    return (
        <FormDialog
            open
            title={`Link ${provider.name} account`}
            submitLabel={`Continue to ${chosen.name}`}
            busy={control.pending}
            onSubmit={() =>
                control.start(provider, { purpose: "link", reauthenticate_with: chosen.id, return_to: ACCOUNT_PATH })
            }
            onCancel={() => {
                control.reset();
                onClose();
            }}
        >
            {failure !== null && <ErrorBanner message={failure} takeFocus />}
            {held.length > 1 ? (
                <FormField label="Confirm it is you by signing in with">
                    {field => (
                        <Select {...field} value={chosen.id} onChange={e => setChosenId(e.target.value)}>
                            {held.map(candidate => (
                                <option key={candidate.id} value={candidate.id}>
                                    {candidate.name}
                                </option>
                            ))}
                        </Select>
                    )}
                </FormField>
            ) : (
                <p>{`Confirm it is you by signing in with ${chosen.name}.`}</p>
            )}
        </FormDialog>
    );
}

function LinkProvider({ providers, held, hasPassword, onOpen }: LinkProviderProps) {
    const signIn = useProviderSignIn();
    const [linking, setLinking] = useState<IdentityProviderResponse | null>(null);

    if (linking !== null && hasPassword) {
        return <LinkProviderForm provider={linking} control={signIn} onClose={() => setLinking(null)} />;
    }

    if (linking !== null && held.length > 0) {
        return (
            <ConfirmWithProviderForm provider={linking} held={held} control={signIn} onClose={() => setLinking(null)} />
        );
    }

    return (
        <div className={styles.actions}>
            {providers.map(provider => (
                <Button
                    key={provider.id}
                    type="button"
                    onClick={() => {
                        onOpen();
                        setLinking(provider);
                    }}
                >
                    Link {provider.name} account
                </Button>
            ))}
        </div>
    );
}

export function SignInMethodsSection({ user }: SignInMethodsSectionProps) {
    const { identity_providers } = useSiteInfo();
    const identities = useIdentities();
    const [searchParams, setSearchParams] = useSearchParams();

    const callbackProvider = identityProviderName(
        identity_providers,
        searchParams.get(PROVIDER_CALLBACK_PROVIDER_PARAM),
    );
    const callbackFailure =
        callbackProvider === null
            ? null
            : providerCallbackMessage(searchParams.get(PROVIDER_CALLBACK_ERROR_PARAM), "link", callbackProvider);
    const linked = identities.data?.identities;
    const linkable =
        linked === undefined
            ? []
            : identity_providers.filter(provider => !linked.some(identity => identity.provider === provider.id));
    const held =
        linked === undefined
            ? []
            : identity_providers.filter(provider => linked.some(identity => identity.provider === provider.id));
    const canLink = linkable.length > 0 && (user.has_password || held.length > 0);

    return (
        <SettingsRow title="Sign-in methods" headingLevel={2} description="Other accounts you can sign in with.">
            {callbackFailure !== null && <ErrorBanner message={callbackFailure} takeFocus />}
            <IdentityList hasPassword={user.has_password} />
            {canLink && (
                <LinkProvider
                    providers={linkable}
                    held={held}
                    hasPassword={user.has_password}
                    onOpen={() => {
                        setSearchParams(
                            current => {
                                const next = new URLSearchParams(current);
                                next.delete(PROVIDER_CALLBACK_ERROR_PARAM);
                                next.delete(PROVIDER_CALLBACK_PROVIDER_PARAM);

                                return next;
                            },
                            { replace: true },
                        );
                    }}
                />
            )}
        </SettingsRow>
    );
}
