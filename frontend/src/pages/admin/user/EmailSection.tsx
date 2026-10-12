import { type SubmitEvent, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ConfirmDialog } from "../../../components/ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../../components/FormField/TextField";
import { Panel } from "../../../components/Panel/Panel";
import { Toast } from "../../../components/Toast/Toast";
import { emailDeliveryRefusal } from "../../../domain/refusals";
import { useSetUserEmail, useUnverifyUserEmail, useVerifyUserEmail } from "../../../hooks/mutations/admin";
import type { AdminUserDetailResponse } from "../../../types/api";
import styles from "./UserSection.module.css";

interface EmailSectionProps {
    user: AdminUserDetailResponse;
    canChange: boolean;
    canVerify: boolean;
}

interface SectionUserProps {
    user: AdminUserDetailResponse;
}

const EMAIL_MISSING = "Enter the new email address.";

function ChangeEmailForm({ user }: SectionUserProps) {
    const change = useSetUserEmail();
    const [email, setEmail] = useState("");
    const [emailError, setEmailError] = useState<string | null>(null);

    const summary = emailError ?? emailDeliveryRefusal(change.error) ?? change.error?.message ?? null;

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();

        const trimmed = email.trim();

        if (trimmed === "") {
            change.reset();
            setEmailError(EMAIL_MISSING);
            return;
        }

        setEmailError(null);
        change.mutate(
            { userId: user.id, email: trimmed },
            {
                onSuccess: () => {
                    setEmail("");
                },
            },
        );
    }

    return (
        <>
            <form className={styles.form} onSubmit={handleSubmit} noValidate>
                {summary !== null && <ErrorBanner message={summary} takeFocus />}
                <TextField
                    label="New email address"
                    hint="We send a link to the new address. The address changes once that link is used."
                    type="email"
                    value={email}
                    onChange={e => setEmail(e.target.value)}
                    error={emailError}
                    autoComplete="off"
                />
                <div>
                    <Button type="submit" variant="primary" disabled={change.isPending}>
                        Change email address
                    </Button>
                </div>
            </form>
            {change.isSuccess && (
                <Toast variant="success" onDismiss={change.reset}>
                    Link sent. The address changes once it is used.
                </Toast>
            )}
        </>
    );
}

function VerificationControl({ user }: SectionUserProps) {
    const verify = useVerifyUserEmail();
    const unverify = useUnverifyUserEmail();
    const [confirming, setConfirming] = useState(false);

    function openConfirm() {
        unverify.reset();
        setConfirming(true);
    }

    function closeConfirm() {
        unverify.reset();
        setConfirming(false);
    }

    function handleUnverify() {
        unverify.mutate(
            { userId: user.id },
            {
                onSuccess: () => {
                    setConfirming(false);
                },
            },
        );
    }

    const dialogBody = (
        <div className={styles.form}>
            <p>
                <bdi>{user.display_name}</bdi> is signed out everywhere and cannot sign in until they verify the address
                again.
            </p>
            {unverify.error !== null && <ErrorBanner message={unverify.error.message} takeFocus />}
        </div>
    );

    return (
        <>
            {verify.error !== null && <ErrorBanner message={verify.error.message} takeFocus />}
            <div className={styles.actions}>
                {user.email_verified ? (
                    <Button type="button" onClick={openConfirm}>
                        Mark unverified
                    </Button>
                ) : (
                    <Button
                        type="button"
                        onClick={() => verify.mutate({ userId: user.id })}
                        disabled={verify.isPending}
                    >
                        Mark verified
                    </Button>
                )}
            </div>
            <ConfirmDialog
                open={confirming}
                title="Mark email unverified"
                body={dialogBody}
                confirmLabel="Mark unverified"
                destructive
                busy={unverify.isPending}
                onConfirm={handleUnverify}
                onCancel={closeConfirm}
            />
            {verify.isSuccess && (
                <Toast variant="success" onDismiss={verify.reset}>
                    Email marked verified.
                </Toast>
            )}
            {unverify.isSuccess && (
                <Toast variant="success" onDismiss={unverify.reset}>
                    Email marked unverified.
                </Toast>
            )}
        </>
    );
}

export function EmailSection({ user, canChange, canVerify }: EmailSectionProps) {
    return (
        <Panel title="Email address" headingLevel={3}>
            {canVerify && <VerificationControl user={user} />}
            {canChange && <ChangeEmailForm user={user} />}
        </Panel>
    );
}
