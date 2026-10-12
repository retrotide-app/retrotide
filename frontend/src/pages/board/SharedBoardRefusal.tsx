import { useLocation } from "react-router";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { returnState } from "../../domain/returnTo";
import { useAuth } from "../../hooks/useAuth";
import styles from "./BoardPage.module.css";

interface SharedBoardRefusalProps {
    message: string;
    rejoinPath: string | null;
}

export function SharedBoardRefusal({ message, rejoinPath }: SharedBoardRefusalProps) {
    const { user } = useAuth();
    const location = useLocation();

    return (
        <div className={styles.refusal}>
            <ErrorBanner message={message} />
            <div className={styles.refusalActions}>
                {rejoinPath !== null && (
                    <ButtonLink to={rejoinPath} variant="primary">
                        Join again
                    </ButtonLink>
                )}
                {user === null && (
                    <ButtonLink to="/login" state={returnState(location.pathname, location.search)}>
                        Sign in
                    </ButtonLink>
                )}
            </div>
        </div>
    );
}
