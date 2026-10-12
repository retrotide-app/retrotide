import { toDataURL } from "qrcode";
import { useEffect, useState } from "react";
import { Banner } from "../../components/Banner/Banner";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../components/FormDialog/FormDialog";
import { TextField } from "../../components/FormField/TextField";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import { recoveryCodesText, TWO_FACTOR_CODE_LENGTH } from "../../domain/twoFactor";
import { useBeginTwoFactor, useConfirmTwoFactor, useDisableTwoFactor } from "../../hooks/mutations/twoFactor";
import { useTwoFactorStatus } from "../../hooks/queries/twoFactor";
import { useCopyText } from "../../hooks/useCopyText";
import { useSiteInfo } from "../../hooks/useSiteInfo";
import type { TwoFactorEnrolmentResponse } from "../../types/api";
import type { SessionUser } from "../../types/app";
import { errorCode, errorMessage } from "../../utils/errorMessage";
import styles from "./AccountSection.module.css";

interface TwoFactorSectionProps {
    user: SessionUser;
}

interface EnrolmentQrProps {
    uri: string;
}

interface QrDrawing {
    url: string | null;
    failure: string | null;
}

interface ConfirmEnrolmentProps {
    enrolment: TwoFactorEnrolmentResponse;
    onClose: () => void;
    onEnabled: (recoveryCodes: string[]) => void;
}

interface TurnOnTwoFactorProps {
    onEnabled: (recoveryCodes: string[]) => void;
}

interface RecoveryCodesDialogProps {
    recoveryCodes: string[];
    onDone: () => void;
}

const TITLE = "Two-factor authentication";

const PASSWORD_FIRST = "Set a password first, then turn on two-factor authentication.";

const PASSWORD_MISSING = "Enter your current password.";

const CODE_MISSING = `Enter the ${TWO_FACTOR_CODE_LENGTH}-digit code from your authenticator app.`;

const CODE_HINT = `The ${TWO_FACTOR_CODE_LENGTH}-digit code your authenticator app shows now.`;

const TURN_OFF_CODE_MISSING = "Enter the code from your authenticator app, or one of your recovery codes.";

const TURN_OFF_CODE_HINT = `The ${TWO_FACTOR_CODE_LENGTH}-digit code your authenticator app shows now, or one of your recovery codes.`;

const RECOVERY_TITLE = "Save your recovery codes";

const RECOVERY_WARNING =
    "This is the only time these codes are shown. Save them somewhere safe, such as a password manager, before you close this.";

const RECOVERY_USE =
    "If you lose your authenticator app, sign in with one of these codes instead. Each code works once. Turning two-factor authentication off and on again gives you new codes.";

const SIGNED_OUT_ELSEWHERE = "Every other device you were signed in on has been signed out.";

const CHECKING = "Checking whether two-factor authentication is on...";

const ON_DESCRIPTION = "On. When you sign in, you also enter a code from your authenticator app.";

const OFF_DESCRIPTION =
    "Off. Turn it on to ask for a code from an authenticator app as well as your password when you sign in.";

const ENROLMENT_STEPS =
    "Scan the QR code with your authenticator app, or type the key into it by hand. Then enter the code it shows. Turning it on signs you out on every other device.";

const QR_SIZE = 200;

const QR_MARGIN = 1;

function EnrolmentQr({ uri }: EnrolmentQrProps) {
    const [drawing, setDrawing] = useState<QrDrawing>({ url: null, failure: null });

    useEffect(() => {
        let active = true;

        toDataURL(uri, { width: QR_SIZE, margin: QR_MARGIN })
            .then(url => {
                if (active) {
                    setDrawing({ url, failure: null });
                }
            })
            .catch((failure: unknown) => {
                if (active) {
                    setDrawing({ url: null, failure: errorMessage(failure, "the QR code library failed") });
                }
            });

        return () => {
            active = false;
        };
    }, [uri]);

    if (drawing.failure !== null) {
        return (
            <ErrorBanner
                message={`The QR code could not be drawn (${drawing.failure}). Type the key below into your authenticator app instead.`}
            />
        );
    }

    if (drawing.url === null) {
        return <p className={styles.text}>Drawing the QR code...</p>;
    }

    return (
        <img
            src={drawing.url}
            alt="QR code that adds this account to an authenticator app"
            width={QR_SIZE}
            height={QR_SIZE}
        />
    );
}

function RecoveryCodesDialog({ recoveryCodes, onDone }: RecoveryCodesDialogProps) {
    const copy = useCopyText();
    const { site_name } = useSiteInfo();

    return (
        <FormDialog
            open
            title={RECOVERY_TITLE}
            submitLabel="I've saved my codes"
            cancelLabel="Close"
            extraActions={
                <Button type="button" onClick={() => copy.copy(recoveryCodesText(site_name, recoveryCodes))}>
                    Copy codes
                </Button>
            }
            onSubmit={onDone}
            onCancel={onDone}
        >
            <Banner tone="warning">{RECOVERY_WARNING}</Banner>
            <p className={styles.text}>{RECOVERY_USE}</p>
            <ol className={styles.codes} aria-label="Recovery codes">
                {recoveryCodes.map(recoveryCode => (
                    <li key={recoveryCode}>
                        <code className={styles.address}>{recoveryCode}</code>
                    </li>
                ))}
            </ol>
            <p className={styles.text}>{SIGNED_OUT_ELSEWHERE}</p>
            {copy.copied && (
                <Banner tone="success" role="status">
                    Codes copied.
                </Banner>
            )}
            {copy.error !== null && <ErrorBanner message={copy.error.message} takeFocus />}
        </FormDialog>
    );
}

function ConfirmEnrolment({ enrolment, onClose, onEnabled }: ConfirmEnrolmentProps) {
    const [code, setCode] = useState("");
    const [problem, setProblem] = useState<string | null>(null);
    const confirm = useConfirmTwoFactor(onEnabled);
    const { site_name } = useSiteInfo();

    const failure = confirm.error?.message ?? null;
    const codeError = problem ?? (errorCode(confirm.error) === "invalid_code" ? failure : null);
    const summary = problem ?? failure;

    function handleSubmit() {
        const entered = code.trim();

        if (entered === "") {
            confirm.reset();
            setProblem(CODE_MISSING);
            return;
        }

        setProblem(null);
        confirm.mutate({ code: entered }, { onSuccess: onClose });
    }

    return (
        <FormDialog
            open
            title={`Add ${site_name} to your authenticator app`}
            submitLabel="Turn on"
            busy={confirm.isPending}
            onSubmit={handleSubmit}
            onCancel={() => {
                confirm.reset();
                onClose();
            }}
        >
            {summary !== null && <ErrorBanner message={summary} takeFocus />}
            <p className={styles.text}>{ENROLMENT_STEPS}</p>
            <EnrolmentQr uri={enrolment.otpauth_uri} />
            <p className={styles.text}>
                Key: <code className={styles.address}>{enrolment.secret}</code>
            </p>
            <TextField
                label="Code"
                hint={CODE_HINT}
                inputMode="numeric"
                autoComplete="one-time-code"
                error={codeError}
                value={code}
                onChange={e => setCode(e.target.value)}
                required
            />
        </FormDialog>
    );
}

function TurnOnTwoFactor({ onEnabled }: TurnOnTwoFactorProps) {
    const [open, setOpen] = useState(false);
    const [password, setPassword] = useState("");
    const [problem, setProblem] = useState<string | null>(null);
    const [enrolment, setEnrolment] = useState<TwoFactorEnrolmentResponse | null>(null);
    const begin = useBeginTwoFactor();

    const code = errorCode(begin.error);
    const failure = code === "password_required" ? PASSWORD_FIRST : (begin.error?.message ?? null);
    const passwordError = problem ?? (code === "incorrect_password" ? failure : null);
    const summary = problem ?? failure;

    function close() {
        setOpen(false);
        setPassword("");
        setProblem(null);
        begin.reset();
    }

    function handleSubmit() {
        if (password === "") {
            begin.reset();
            setProblem(PASSWORD_MISSING);
            return;
        }

        setProblem(null);
        begin.mutate(
            { current_password: password },
            {
                onSuccess: started => {
                    close();
                    setEnrolment(started);
                },
            },
        );
    }

    return (
        <>
            <div className={styles.actions}>
                <Button
                    type="button"
                    onClick={() => {
                        begin.reset();
                        setProblem(null);
                        setOpen(true);
                    }}
                >
                    Turn on
                </Button>
            </div>
            <FormDialog
                open={open}
                title="Turn on two-factor authentication"
                submitLabel="Continue"
                busy={begin.isPending}
                onSubmit={handleSubmit}
                onCancel={close}
            >
                {summary !== null && <ErrorBanner message={summary} takeFocus />}
                <TextField
                    label="Current password"
                    hint="Enter your password to set up two-factor authentication."
                    type="password"
                    autoComplete="current-password"
                    error={passwordError}
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    required
                />
            </FormDialog>
            {enrolment !== null && (
                <ConfirmEnrolment enrolment={enrolment} onClose={() => setEnrolment(null)} onEnabled={onEnabled} />
            )}
        </>
    );
}

function TurnOffTwoFactor() {
    const [open, setOpen] = useState(false);
    const [code, setCode] = useState("");
    const [problem, setProblem] = useState<string | null>(null);
    const disable = useDisableTwoFactor();

    const failure = disable.error?.message ?? null;
    const codeError = problem ?? (errorCode(disable.error) === "invalid_code" ? failure : null);
    const summary = problem ?? failure;

    function close() {
        setOpen(false);
        setCode("");
        setProblem(null);
        disable.reset();
    }

    function handleSubmit() {
        const entered = code.trim();

        if (entered === "") {
            disable.reset();
            setProblem(TURN_OFF_CODE_MISSING);
            return;
        }

        setProblem(null);
        disable.mutate({ code: entered }, { onSuccess: close });
    }

    return (
        <>
            <div className={styles.actions}>
                <Button
                    type="button"
                    onClick={() => {
                        disable.reset();
                        setProblem(null);
                        setOpen(true);
                    }}
                >
                    Turn off
                </Button>
            </div>
            <FormDialog
                open={open}
                title="Turn off two-factor authentication"
                submitLabel="Turn off"
                destructive
                busy={disable.isPending}
                onSubmit={handleSubmit}
                onCancel={close}
            >
                {summary !== null && <ErrorBanner message={summary} takeFocus />}
                <TextField
                    label="Code"
                    hint={TURN_OFF_CODE_HINT}
                    autoComplete="one-time-code"
                    error={codeError}
                    value={code}
                    onChange={e => setCode(e.target.value)}
                    required
                />
            </FormDialog>
        </>
    );
}

function TwoFactorRow({ user, onEnabled }: TwoFactorSectionProps & TurnOnTwoFactorProps) {
    const status = useTwoFactorStatus();

    if (status.error !== null) {
        return (
            <SettingsRow title={TITLE} headingLevel={2}>
                <LoadFailure message={status.error.message} onRetry={status.refetch} />
            </SettingsRow>
        );
    }

    if (status.data === undefined) {
        return <SettingsRow title={TITLE} headingLevel={2} description={CHECKING} />;
    }

    if (status.data.enabled) {
        return (
            <SettingsRow title={TITLE} headingLevel={2} description={ON_DESCRIPTION}>
                <TurnOffTwoFactor />
            </SettingsRow>
        );
    }

    if (!user.has_password) {
        return <SettingsRow title={TITLE} headingLevel={2} description={PASSWORD_FIRST} />;
    }

    return (
        <SettingsRow title={TITLE} headingLevel={2} description={OFF_DESCRIPTION}>
            <TurnOnTwoFactor onEnabled={onEnabled} />
        </SettingsRow>
    );
}

export function TwoFactorSection({ user }: TwoFactorSectionProps) {
    const [recoveryCodes, setRecoveryCodes] = useState<string[] | null>(null);

    return (
        <>
            <TwoFactorRow user={user} onEnabled={setRecoveryCodes} />
            {recoveryCodes !== null && (
                <RecoveryCodesDialog recoveryCodes={recoveryCodes} onDone={() => setRecoveryCodes(null)} />
            )}
        </>
    );
}
