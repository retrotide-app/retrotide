import { useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { Panel } from "../../../components/Panel/Panel";
import { Toast } from "../../../components/Toast/Toast";
import { useLockUser, useUnlockUser } from "../../../hooks/mutations/admin";
import type { AdminUserDetailResponse } from "../../../types/api";
import { ReasonDialog } from "./ReasonDialog";

interface LockSectionProps {
    user: AdminUserDetailResponse;
}

export function LockSection({ user }: LockSectionProps) {
    const lock = useLockUser();
    const unlock = useUnlockUser();
    const [confirming, setConfirming] = useState(false);

    function openConfirm() {
        lock.reset();
        setConfirming(true);
    }

    function closeConfirm() {
        lock.reset();
        setConfirming(false);
    }

    function handleLock(reason: string) {
        lock.mutate(
            { userId: user.id, reason },
            {
                onSuccess: () => {
                    setConfirming(false);
                },
            },
        );
    }

    const description = user.locked
        ? "This user can sign in and read, but cannot make changes until you unlock them."
        : "A locked user can sign in and read, but cannot make changes.";

    return (
        <Panel title="Lock" headingLevel={3} description={description}>
            {unlock.error !== null && <ErrorBanner message={unlock.error.message} takeFocus />}
            <div>
                {user.locked ? (
                    <Button
                        type="button"
                        onClick={() => unlock.mutate({ userId: user.id })}
                        disabled={unlock.isPending}
                    >
                        Unlock user
                    </Button>
                ) : (
                    <Button type="button" variant="danger" onClick={openConfirm}>
                        Lock user
                    </Button>
                )}
            </div>
            {confirming && (
                <ReasonDialog
                    title="Lock user"
                    body={
                        <>
                            <bdi>{user.display_name}</bdi> can still sign in and read, but cannot make changes until you
                            unlock them.
                        </>
                    }
                    confirmLabel="Lock user"
                    busy={lock.isPending}
                    error={lock.error}
                    onConfirm={handleLock}
                    onCancel={closeConfirm}
                />
            )}
            {lock.isSuccess && (
                <Toast variant="success" onDismiss={lock.reset}>
                    User locked.
                </Toast>
            )}
            {unlock.isSuccess && (
                <Toast variant="success" onDismiss={unlock.reset}>
                    User unlocked.
                </Toast>
            )}
        </Panel>
    );
}
