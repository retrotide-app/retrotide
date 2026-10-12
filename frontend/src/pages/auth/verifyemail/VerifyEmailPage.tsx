import { useEffect, useRef, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ButtonLink } from "../../../components/Button/ButtonLink";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { AuthLayout } from "../../../components/layout/AuthLayout/AuthLayout";
import { TextLink } from "../../../components/TextLink/TextLink";
import { useVerifyEmail } from "../../../hooks/mutations/auth";
import { usePageTitle } from "../../../hooks/usePageTitle";
import { clearLocationToken, readLocationToken } from "../../../platform/location";
import { errorMessage, errorStatus } from "../../../utils/errorMessage";
import { AuthNotice } from "../AuthNotice/AuthNotice";
import { ResendVerification } from "../ResendVerification/ResendVerification";
import styles from "../AuthForm.module.css";

const TITLE = "Verify your email address";

function worthRetrying(error: Error): boolean {
    const status = errorStatus(error);

    return status === null || status === 429 || status >= 500;
}

export function VerifyEmailPage() {
    usePageTitle(TITLE);

    const [token] = useState(readLocationToken);
    const verification = useVerifyEmail();
    const { mutate } = verification;
    const started = useRef(false);

    useEffect(() => {
        clearLocationToken();

        if (started.current || token === "") {
            return;
        }

        started.current = true;
        mutate({ token });
    }, [mutate, token]);

    const footer = (
        <p>
            <TextLink to="/login">Back to sign in</TextLink>
        </p>
    );

    if (token === "") {
        return (
            <AuthLayout title={TITLE} footer={footer}>
                <ErrorBanner message="This link is missing its code. Open the link from your email again." takeFocus />
                <ResendVerification />
            </AuthLayout>
        );
    }

    if (verification.isSuccess) {
        return (
            <AuthLayout title={TITLE}>
                <AuthNotice>Your email address is verified.</AuthNotice>
                <ButtonLink to="/login" variant="primary" className={styles.wide}>
                    Sign in
                </ButtonLink>
            </AuthLayout>
        );
    }

    if (verification.error !== null) {
        return (
            <AuthLayout title={TITLE} footer={footer}>
                <ErrorBanner
                    message={errorMessage(verification.error, "Your email address could not be verified.")}
                    takeFocus
                />
                {worthRetrying(verification.error) && (
                    <div className={`${styles.actions} ${styles.gapBelow}`}>
                        <Button
                            type="button"
                            onClick={() => {
                                mutate({ token });
                            }}
                            className={styles.wide}
                        >
                            Try again
                        </Button>
                    </div>
                )}
                <ResendVerification />
            </AuthLayout>
        );
    }

    return (
        <AuthLayout title={TITLE}>
            <p role="status" className={styles.lead}>
                Checking your link...
            </p>
        </AuthLayout>
    );
}
