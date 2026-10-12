import { useState } from "react";
import { Banner } from "../../components/Banner/Banner";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FormDialog } from "../../components/FormDialog/FormDialog";
import { TextField } from "../../components/FormField/TextField";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import { emailDeliveryRefusal } from "../../domain/refusals";
import { useSetEmail } from "../../hooks/mutations/auth";
import type { SessionUser } from "../../types/app";
import { errorCode, errorStatus } from "../../utils/errorMessage";
import styles from "./AccountSection.module.css";

interface EmailSectionProps {
    user: SessionUser;
    pendingEmail: string | null;
    onChangeRequested: (email: string) => void;
}

interface EmailChangeProblems {
    email?: string;
    password?: string;
}

const EMAIL_FIELD_STATUSES: ReadonlySet<number | null> = new Set([400, 409]);

function findProblems(email: string, password: string): EmailChangeProblems {
    const problems: EmailChangeProblems = {};

    if (email === "") {
        problems.email = "Enter the new email address.";
    }

    if (password === "") {
        problems.password = "Enter your current password.";
    }

    return problems;
}

export function EmailSection({ user, pendingEmail, onChangeRequested }: EmailSectionProps) {
    const [open, setOpen] = useState(false);
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [problems, setProblems] = useState<EmailChangeProblems>({});
    const change = useSetEmail();

    const failure = emailDeliveryRefusal(change.error) ?? change.error?.message ?? null;
    const listed = Object.values(problems).join(" ");
    const summary = listed === "" ? failure : listed;
    const passwordError = problems.password ?? (errorCode(change.error) === "incorrect_password" ? failure : null);
    const emailError = problems.email ?? (EMAIL_FIELD_STATUSES.has(errorStatus(change.error)) ? failure : null);

    function close() {
        setOpen(false);
        setEmail("");
        setPassword("");
        setProblems({});
        change.reset();
    }

    function handleSubmit() {
        const requested = email.trim();
        const found = findProblems(requested, password);
        setProblems(found);

        if (Object.keys(found).length > 0) {
            change.reset();
            return;
        }

        change.mutate(
            { email: requested, password },
            {
                onSuccess: () => {
                    setOpen(false);
                    setEmail("");
                    setPassword("");
                    onChangeRequested(requested);
                },
            },
        );
    }

    const description = (
        <>
            Your email address is <bdi className={styles.address}>{user.email}</bdi>.
        </>
    );

    return (
        <SettingsRow title="Email address" headingLevel={2} description={description}>
            {pendingEmail !== null && (
                <Banner tone="info" role="status">
                    We sent a link to <bdi>{pendingEmail}</bdi>. Your email address changes to it when you use the link.
                </Banner>
            )}
            {user.has_password ? (
                <div className={styles.actions}>
                    <Button
                        type="button"
                        onClick={() => {
                            change.reset();
                            setProblems({});
                            setOpen(true);
                        }}
                    >
                        Change email address
                    </Button>
                </div>
            ) : (
                <p className={styles.text}>Set a password before you change your email address.</p>
            )}
            <FormDialog
                open={open}
                title="Change email address"
                submitLabel="Send link"
                busy={change.isPending}
                onSubmit={handleSubmit}
                onCancel={close}
            >
                <p className={styles.text}>
                    We will email a link to the new address. Your address changes when you use it.
                </p>
                {summary !== null && <ErrorBanner message={summary} takeFocus />}
                <TextField
                    label="New email address"
                    type="email"
                    autoComplete="email"
                    error={emailError}
                    value={email}
                    onChange={e => setEmail(e.target.value)}
                    required
                />
                <TextField
                    label="Current password"
                    type="password"
                    autoComplete="current-password"
                    error={passwordError}
                    value={password}
                    onChange={e => setPassword(e.target.value)}
                    required
                />
            </FormDialog>
        </SettingsRow>
    );
}
