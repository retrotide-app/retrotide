import { useEffect, useId, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { Button } from "../../../components/Button/Button";
import { ThemeSelect } from "../../../components/layout/ThemeSelect/ThemeSelect";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { TextLink } from "../../../components/TextLink/TextLink";
import { Toast } from "../../../components/Toast/Toast";
import { accountInitials } from "../../../domain/displayName";
import { STAFF_ROLE_LABELS } from "../../../domain/staffRoles";
import { useLogout } from "../../../hooks/mutations/auth";
import { useAuth } from "../../../hooks/useAuth";
import styles from "./AccountMenu.module.css";

interface AccountMenuProps {
    backTo: string;
}

export function AccountMenu({ backTo }: AccountMenuProps) {
    const { user, siteRole } = useAuth();
    const logout = useLogout();
    const navigate = useNavigate();
    const panelId = useId();
    const rootRef = useRef<HTMLDivElement>(null);
    const triggerRef = useRef<HTMLButtonElement>(null);
    const [open, setOpen] = useState(false);

    useEffect(() => {
        if (!open) {
            return;
        }

        function handlePointerDown(event: PointerEvent) {
            if (event.target instanceof Node && rootRef.current?.contains(event.target)) {
                return;
            }

            setOpen(false);
        }

        function handleKeyDown(event: KeyboardEvent) {
            if (event.key !== "Escape") {
                return;
            }

            setOpen(false);
            triggerRef.current?.focus();
        }

        document.addEventListener("pointerdown", handlePointerDown);
        document.addEventListener("keydown", handleKeyDown);

        return () => {
            document.removeEventListener("pointerdown", handlePointerDown);
            document.removeEventListener("keydown", handleKeyDown);
        };
    }, [open]);

    if (user === null) {
        return null;
    }

    const named = user.display_name.trim() !== "";

    return (
        <div ref={rootRef} className={styles.root}>
            <Button
                ref={triggerRef}
                type="button"
                size="small"
                className={`${styles.trigger}${named ? "" : ` ${styles.avatarOnly}`}`}
                aria-label={named ? undefined : user.email}
                aria-expanded={open}
                aria-controls={panelId}
                onClick={() => setOpen(current => !current)}
            >
                <span className={styles.avatar} aria-hidden="true">
                    {accountInitials(user.display_name, user.email)}
                </span>
                {named && (
                    <span className={styles.triggerName}>
                        <bdi>{user.display_name}</bdi>
                    </span>
                )}
            </Button>

            {open && (
                <div id={panelId} className={styles.panel}>
                    <div className={styles.who}>
                        <span className={styles.name}>
                            <bdi>{user.display_name}</bdi>
                        </span>
                        <span className={styles.email}>
                            <bdi>{user.email}</bdi>
                        </span>
                        {siteRole !== null && <StatusTag tone="neutral">{STAFF_ROLE_LABELS[siteRole]}</StatusTag>}
                    </div>
                    <div className={styles.row}>
                        <ThemeSelect />
                    </div>
                    <nav className={styles.links} aria-label="Account">
                        <TextLink to={backTo} onClick={() => setOpen(false)}>
                            Back to app
                        </TextLink>
                        <TextLink to="/account" onClick={() => setOpen(false)}>
                            Your account
                        </TextLink>
                    </nav>
                    <div className={styles.row}>
                        <Button
                            type="button"
                            size="small"
                            disabled={logout.isPending}
                            onClick={() => {
                                logout.mutate(undefined, {
                                    onSuccess: () => {
                                        navigate("/login", { replace: true });
                                    },
                                });
                            }}
                        >
                            Sign out
                        </Button>
                    </div>
                </div>
            )}

            {logout.error !== null && (
                <Toast variant="error" onDismiss={logout.reset}>
                    {logout.error.message}
                </Toast>
            )}
        </div>
    );
}
