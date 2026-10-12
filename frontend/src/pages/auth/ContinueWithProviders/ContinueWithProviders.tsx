import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { providerStartFailure } from "../../../domain/providerStart";
import type { ProviderSignIn } from "../../../hooks/useProviderSignIn";
import type { IdentityProviderResponse } from "../../../types/api";
import styles from "./ContinueWithProviders.module.css";

interface ContinueWithProvidersProps {
    providers: readonly IdentityProviderResponse[];
    control: ProviderSignIn;
    onStart: (provider: IdentityProviderResponse) => void;
    showDivider?: boolean;
}

export function ContinueWithProviders({ providers, control, onStart, showDivider = true }: ContinueWithProvidersProps) {
    const failure =
        control.error === null || control.provider === null
            ? null
            : providerStartFailure(control.error, "sign_in", control.provider.name);

    return (
        <div className={styles.providers}>
            {failure !== null && <ErrorBanner message={failure} takeFocus />}
            <div className={styles.buttons}>
                {providers.map(provider => (
                    <Button
                        key={provider.id}
                        type="button"
                        className={styles.button}
                        onClick={() => onStart(provider)}
                        disabled={control.pending}
                    >
                        Continue with {provider.name}
                    </Button>
                ))}
            </div>
            {showDivider && <p className={styles.divider}>or</p>}
        </div>
    );
}
