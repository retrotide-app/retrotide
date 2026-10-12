import { facilitationRefusal } from "../../domain/board/facilitation";
import { useShowColumn } from "../../hooks/mutations/facilitation";
import { Button } from "../Button/Button";
import { ErrorBanner } from "../ErrorBanner/ErrorBanner";
import styles from "./HiddenColumnNotice.module.css";

interface HiddenColumnNoticeProps {
    boardId: string;
    columnId: string;
    onShowForMe: () => void;
}

export function HiddenColumnNotice({ boardId, columnId, onShowForMe }: HiddenColumnNoticeProps) {
    const show = useShowColumn(boardId);

    return (
        <div className={styles.notice}>
            <p className={styles.text}>Hidden from participants</p>
            {show.error !== null && <ErrorBanner message={facilitationRefusal(show.error)} />}
            <div className={styles.actions}>
                <Button type="button" size="small" variant="secondary" onClick={onShowForMe}>
                    Show for me
                </Button>
                <Button
                    type="button"
                    size="small"
                    variant="primary"
                    disabled={show.isPending}
                    onClick={() => show.mutate({ columnId })}
                >
                    Show to everyone
                </Button>
            </div>
        </div>
    );
}
