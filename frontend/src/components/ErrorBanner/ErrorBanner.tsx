import { useEffect, useRef } from "react";
import styles from "./ErrorBanner.module.css";

interface ErrorBannerProps {
    message: string;
    takeFocus?: boolean;
}

export function ErrorBanner({ message, takeFocus = false }: ErrorBannerProps) {
    const bannerRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (!takeFocus) {
            return;
        }

        bannerRef.current?.focus();
    }, [takeFocus, message]);

    return (
        <div ref={bannerRef} role="alert" tabIndex={takeFocus ? -1 : undefined} className={styles.banner} dir="auto">
            {message}
        </div>
    );
}
