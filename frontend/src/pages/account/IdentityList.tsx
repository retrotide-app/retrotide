import { useState } from "react";
import { Banner } from "../../components/Banner/Banner";
import { Button } from "../../components/Button/Button";
import { ConfirmDialog } from "../../components/ConfirmDialog/ConfirmDialog";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { useUnlinkIdentity } from "../../hooks/mutations/identity";
import { useIdentities } from "../../hooks/queries/identity";
import type { IdentityResponse } from "../../types/api";
import { errorCode } from "../../utils/errorMessage";
import { formatExactDateTime } from "../../utils/time";
import styles from "./AccountSection.module.css";

interface IdentityListProps {
    hasPassword: boolean;
}

const ONLY_SIGN_IN_METHOD = "This is your only way to sign in. Set a password before you remove it.";

export function IdentityList({ hasPassword }: IdentityListProps) {
    const identities = useIdentities();
    const unlink = useUnlinkIdentity();
    const [removing, setRemoving] = useState<IdentityResponse | null>(null);

    if (identities.error !== null) {
        return <LoadFailure message={identities.error.message} onRetry={identities.refetch} />;
    }

    if (identities.data === undefined) {
        return <p className={styles.text}>Loading sign-in methods...</p>;
    }

    const linked = identities.data.identities;
    const onlyWayIn = !hasPassword && linked.length === 1;
    const removingName = removing === null ? "" : removing.provider_name;
    const removedName = unlink.variables === undefined ? "" : unlink.variables.provider_name;

    let unlinkFailure: string | null = null;
    if (unlink.error !== null) {
        unlinkFailure = errorCode(unlink.error) === "last_sign_in_method" ? ONLY_SIGN_IN_METHOD : unlink.error.message;
    }

    return (
        <>
            {linked.length === 0 ? (
                <p className={styles.text}>No other accounts are linked.</p>
            ) : (
                <ul className={styles.list} aria-label="Linked sign-in methods">
                    {linked.map(identity => (
                        <li key={identity.provider} className={styles.item}>
                            <div className={styles.itemText}>
                                <span className={styles.itemName}>{identity.provider_name}</span>
                                <bdi className={styles.itemMeta}>{identity.email}</bdi>
                                <span className={styles.itemMeta}>
                                    Linked {formatExactDateTime(identity.created_at)}
                                </span>
                            </div>
                            {onlyWayIn ? (
                                <p className={styles.itemMeta}>{ONLY_SIGN_IN_METHOD}</p>
                            ) : (
                                <Button
                                    type="button"
                                    size="small"
                                    aria-label={`Remove ${identity.provider_name} sign-in for ${identity.email}`}
                                    onClick={() => {
                                        unlink.reset();
                                        setRemoving(identity);
                                    }}
                                >
                                    Remove
                                </Button>
                            )}
                        </li>
                    ))}
                </ul>
            )}
            {unlinkFailure !== null && <ErrorBanner message={unlinkFailure} takeFocus />}
            {unlink.isSuccess && (
                <Banner tone="success" role="status">
                    {removedName} sign-in removed.
                </Banner>
            )}
            <ConfirmDialog
                open={removing !== null}
                title={`Remove ${removingName} sign-in?`}
                body={
                    <p>
                        You will no longer be able to sign in with <bdi>{removing?.email}</bdi>.
                    </p>
                }
                confirmLabel="Remove"
                destructive
                busy={unlink.isPending}
                onConfirm={() => {
                    if (removing === null) {
                        return;
                    }

                    unlink.mutate(removing, {
                        onSettled: () => {
                            setRemoving(null);
                        },
                    });
                }}
                onCancel={() => {
                    setRemoving(null);
                }}
            />
        </>
    );
}
