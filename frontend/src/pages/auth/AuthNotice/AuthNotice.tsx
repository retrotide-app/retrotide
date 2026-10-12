import { type ReactNode, useEffect, useRef } from "react";
import styles from "./AuthNotice.module.css";

interface AuthNoticeProps {
    children: ReactNode;
}

export function AuthNotice({ children }: AuthNoticeProps) {
    const noticeRef = useRef<HTMLParagraphElement>(null);

    useEffect(() => {
        noticeRef.current?.focus();
    }, []);

    return (
        <p ref={noticeRef} role="status" tabIndex={-1} className={styles.notice}>
            {children}
        </p>
    );
}
