import { type SubmitEvent, useState } from "react";
import { useLocation, useNavigate } from "react-router";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { AuthLayout } from "../../../components/layout/AuthLayout/AuthLayout";
import { TextLink } from "../../../components/TextLink/TextLink";
import { ToggleSwitch } from "../../../components/ToggleSwitch/ToggleSwitch";
import { isSafeReturnPath } from "../../../domain/returnTo";
import { TRUSTED_DEVICE_DAYS, TWO_FACTOR_CODE_LENGTH } from "../../../domain/twoFactor";
import { useVerifyTwoFactor } from "../../../hooks/mutations/twoFactor";
import { usePageTitle } from "../../../hooks/usePageTitle";
import { useReturnPath } from "../../../hooks/useReturnPath";
import { errorCode, errorMessage } from "../../../utils/errorMessage";
import styles from "../AuthForm.module.css";

const TITLE = "Two-factor authentication";

const LOGIN_PATH = "/login";

const CHALLENGE_EXPIRED = "two_factor_expired";

const INVALID_CODE = "invalid_code";

const EXPIRED_MESSAGE = "Signing in took too long, so this sign-in has ended. Sign in again to get a fresh one.";

const CODE_MISSING = `Enter the ${TWO_FACTOR_CODE_LENGTH}-digit code from your authenticator app.`;

const RECOVERY_CODE_MISSING = "Enter one of the recovery codes you saved when you turned on two-factor authentication.";

const VERIFY_FAILED = "The code could not be checked. Try again.";

function landingPath(location: string | undefined, fallback: string): string {
    if (location !== undefined && isSafeReturnPath(location)) {
        return location;
    }

    return fallback;
}

export function TwoFactorPage() {
    usePageTitle(TITLE);

    const returnPath = useReturnPath();
    const location = useLocation();
    const navigate = useNavigate();
    const verify = useVerifyTwoFactor();
    const [code, setCode] = useState("");
    const [recovering, setRecovering] = useState(false);
    const [trustDevice, setTrustDevice] = useState(false);
    const [problem, setProblem] = useState<string | null>(null);

    const failureCode = errorCode(verify.error);
    const failure = verify.error === null ? null : errorMessage(verify.error, VERIFY_FAILED);
    const codeError = problem ?? (failureCode === INVALID_CODE ? failure : null);
    const summary = problem ?? failure;

    const footer = (
        <p>
            <TextLink to={LOGIN_PATH} state={location.state}>
                Back to sign in
            </TextLink>
        </p>
    );

    if (failureCode === CHALLENGE_EXPIRED) {
        return (
            <AuthLayout title={TITLE} footer={footer}>
                <ErrorBanner message={EXPIRED_MESSAGE} takeFocus />
                <p className={styles.links}>
                    <TextLink to={LOGIN_PATH} state={location.state}>
                        Sign in again
                    </TextLink>
                </p>
            </AuthLayout>
        );
    }

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        const entered = code.trim();

        if (entered === "") {
            verify.reset();
            setProblem(recovering ? RECOVERY_CODE_MISSING : CODE_MISSING);
            return;
        }

        setProblem(null);
        verify.mutate(
            { code: entered, trust_device: trustDevice },
            {
                onSuccess: verified => {
                    navigate(landingPath(verified.location, returnPath), { replace: true });
                },
            },
        );
    }

    function switchMode() {
        verify.reset();
        setProblem(null);
        setCode("");
        setRecovering(!recovering);
    }

    return (
        <AuthLayout title={TITLE} footer={footer}>
            {summary !== null && <ErrorBanner message={summary} takeFocus />}
            <p className={styles.lead}>
                {recovering
                    ? "Enter one of the recovery codes you saved. Each code works once."
                    : "Enter the code your authenticator app shows for this account."}
            </p>
            <form className={styles.form} onSubmit={handleSubmit} noValidate>
                <TextField
                    key={recovering ? "recovery" : "app"}
                    label={recovering ? "Recovery code" : "Code"}
                    name="code"
                    inputMode={recovering ? "text" : "numeric"}
                    autoComplete={recovering ? "off" : "one-time-code"}
                    value={code}
                    onChange={event => setCode(event.target.value)}
                    error={codeError}
                />
                <ToggleSwitch
                    label="Don't ask again on this device"
                    description={`Skip the code when you sign in here for the next ${TRUSTED_DEVICE_DAYS} days.`}
                    enabled={trustDevice}
                    onChange={setTrustDevice}
                />
                <Button type="submit" variant="primary" className={styles.wide} disabled={verify.isPending}>
                    Verify
                </Button>
            </form>
            <p className={styles.links}>
                <Button type="button" variant="ghost" size="small" onClick={switchMode}>
                    {recovering ? "Use the code from your authenticator app" : "Lost your phone? Use a recovery code"}
                </Button>
            </p>
        </AuthLayout>
    );
}
