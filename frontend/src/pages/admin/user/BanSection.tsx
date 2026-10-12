import { useState } from "react";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { Panel } from "../../../components/Panel/Panel";
import { Toast } from "../../../components/Toast/Toast";
import { useBanUser, useUnbanUser } from "../../../hooks/mutations/admin";
import type { AdminUserDetailResponse } from "../../../types/api";
import { ReasonDialog } from "./ReasonDialog";

interface BanSectionProps {
    user: AdminUserDetailResponse;
}

export function BanSection({ user }: BanSectionProps) {
    const ban = useBanUser();
    const unban = useUnbanUser();
    const [confirming, setConfirming] = useState(false);

    function openConfirm() {
        ban.reset();
        setConfirming(true);
    }

    function closeConfirm() {
        ban.reset();
        setConfirming(false);
    }

    function handleBan(reason: string) {
        ban.mutate(
            { userId: user.id, reason },
            {
                onSuccess: () => {
                    setConfirming(false);
                },
            },
        );
    }

    const description = user.banned
        ? "This user cannot sign in until you unban them."
        : "A banned user is signed out everywhere and cannot sign in.";

    return (
        <Panel title="Ban" headingLevel={3} description={description}>
            {unban.error !== null && <ErrorBanner message={unban.error.message} takeFocus />}
            <div>
                {user.banned ? (
                    <Button type="button" onClick={() => unban.mutate({ userId: user.id })} disabled={unban.isPending}>
                        Unban user
                    </Button>
                ) : (
                    <Button type="button" variant="danger" onClick={openConfirm}>
                        Ban user
                    </Button>
                )}
            </div>
            {confirming && (
                <ReasonDialog
                    title="Ban user"
                    body={
                        <>
                            <bdi>{user.display_name}</bdi> is signed out everywhere and cannot sign in until you unban
                            them.
                        </>
                    }
                    confirmLabel="Ban user"
                    busy={ban.isPending}
                    error={ban.error}
                    onConfirm={handleBan}
                    onCancel={closeConfirm}
                />
            )}
            {ban.isSuccess && (
                <Toast variant="success" onDismiss={ban.reset}>
                    User banned.
                </Toast>
            )}
            {unban.isSuccess && (
                <Toast variant="success" onDismiss={unban.reset}>
                    User unbanned.
                </Toast>
            )}
        </Panel>
    );
}
