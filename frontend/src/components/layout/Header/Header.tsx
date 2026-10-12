import { useNavigate } from "react-router";
import { useLogout } from "../../../hooks/mutations/auth";
import { useAuth } from "../../../hooks/useAuth";
import { Button } from "../../Button/Button";
import { ButtonLink } from "../../Button/ButtonLink";
import { Toast } from "../../Toast/Toast";
import { ThemeSelect } from "../ThemeSelect/ThemeSelect";
import styles from "./Header.module.css";

interface HeaderProps {
    onToggleSidebar: () => void;
    sidebarOpen: boolean;
}

function AccountActions() {
    const { user, loading, error } = useAuth();
    const logout = useLogout();
    const navigate = useNavigate();

    if (loading || (user === null && error !== null)) {
        return null;
    }

    if (user === null) {
        return (
            <ButtonLink to="/login" variant="primary" size="small">
                Sign in
            </ButtonLink>
        );
    }

    return (
        <>
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
            {logout.error !== null && (
                <Toast variant="error" onDismiss={logout.reset}>
                    {logout.error.message}
                </Toast>
            )}
        </>
    );
}

export function Header({ onToggleSidebar, sidebarOpen }: HeaderProps) {
    return (
        <header className={styles.header}>
            <button
                type="button"
                className={styles.menuButton}
                onClick={onToggleSidebar}
                aria-label="Toggle menu"
                aria-controls="app-sidebar"
                aria-expanded={sidebarOpen}
            >
                <span className={styles.menuBar} />
                <span className={styles.menuBar} />
                <span className={styles.menuBar} />
            </button>

            <div className={styles.actions}>
                <ThemeSelect />
                <AccountActions />
            </div>
        </header>
    );
}
