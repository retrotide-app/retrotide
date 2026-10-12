import { UpgradePrompt } from "../../../components/billing/UpgradePrompt/UpgradePrompt";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { readUpgradeRequired } from "../../../domain/entitlements";
import { emailDeliveryRefusal } from "../../../domain/refusals";

interface ActionFailureProps {
    error: Error;
    takeFocus?: boolean;
}

export function ActionFailure({ error, takeFocus = false }: ActionFailureProps) {
    const upgrade = readUpgradeRequired(error);

    if (upgrade !== null) {
        return <UpgradePrompt upgrade={upgrade} takeFocus={takeFocus} />;
    }

    return <ErrorBanner message={emailDeliveryRefusal(error) ?? error.message} takeFocus={takeFocus} />;
}
