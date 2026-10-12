import { type PropsWithChildren, useEffect, useId, useRef } from "react";
import { createPortal } from "react-dom";
import styles from "./Modal.module.css";

interface ModalProps {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    closeOnOverlayClick?: boolean;
}

export function Modal({ isOpen, onClose, title, closeOnOverlayClick = true, children }: PropsWithChildren<ModalProps>) {
    const panelRef = useRef<HTMLDivElement>(null);
    const titleId = useId();

    useEffect(() => {
        if (!isOpen) {
            return;
        }

        function onKeyDown(e: KeyboardEvent) {
            if (e.key === "Escape") {
                onClose();
            }
        }

        document.addEventListener("keydown", onKeyDown);

        return () => {
            document.removeEventListener("keydown", onKeyDown);
        };
    }, [isOpen, onClose]);

    useEffect(() => {
        if (!isOpen) {
            return;
        }

        const previous = document.activeElement as HTMLElement | null;

        panelRef.current?.focus();

        return () => {
            previous?.focus();
        };
    }, [isOpen]);

    if (!isOpen) {
        return null;
    }

    return createPortal(
        <div className={styles.overlay} onClick={closeOnOverlayClick ? onClose : undefined}>
            <div
                ref={panelRef}
                className={styles.modal}
                role="dialog"
                aria-modal="true"
                aria-labelledby={titleId}
                tabIndex={-1}
                onClick={e => e.stopPropagation()}
            >
                <div className={styles.header}>
                    <h3 id={titleId} className={styles.title} dir="auto">
                        {title}
                    </h3>
                    <button type="button" className={styles.close} aria-label="Close" onClick={onClose}>
                        {"✕"}
                    </button>
                </div>
                <div className={styles.body}>{children}</div>
            </div>
        </div>,
        document.body,
    );
}
