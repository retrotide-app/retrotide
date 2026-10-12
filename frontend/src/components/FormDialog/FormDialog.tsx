import { type ReactNode, type SubmitEvent, useEffect, useRef } from "react";
import { Button } from "../Button/Button";
import { Modal } from "../Modal/Modal";
import styles from "./FormDialog.module.css";

interface FormDialogProps {
    open: boolean;
    title: string;
    submitLabel: string;
    cancelLabel?: string;
    destructive?: boolean;
    busy?: boolean;
    extraActions?: ReactNode;
    onSubmit: () => void;
    onCancel: () => void;
    children?: ReactNode;
}

const ALERT_SELECTOR = "[role='alert'][tabindex='-1']";

const FIELD_SELECTOR = "input:not([type='hidden']):not([disabled]), select:not([disabled]), textarea:not([disabled])";

export function FormDialog({
    open,
    title,
    submitLabel,
    cancelLabel = "Cancel",
    destructive = false,
    busy = false,
    extraActions,
    onSubmit,
    onCancel,
    children,
}: FormDialogProps) {
    const formRef = useRef<HTMLFormElement>(null);

    useEffect(() => {
        if (!open) {
            return;
        }

        const form = formRef.current;

        if (form === null) {
            return;
        }

        const target =
            form.querySelector<HTMLElement>(ALERT_SELECTOR) ??
            form.querySelector<HTMLElement>(FIELD_SELECTOR) ??
            form.querySelector<HTMLElement>("button");

        target?.focus();
    }, [open]);

    function handleCancel() {
        if (busy) {
            return;
        }

        onCancel();
    }

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        if (busy) {
            return;
        }

        onSubmit();
    }

    return (
        <Modal isOpen={open} onClose={handleCancel} title={title}>
            <form ref={formRef} className={styles.form} onSubmit={handleSubmit} aria-label={title} noValidate>
                <div className={styles.fields}>{children}</div>
                <div className={styles.actions} aria-busy={busy}>
                    <Button type="button" variant="secondary" onClick={handleCancel} disabled={busy}>
                        {cancelLabel}
                    </Button>
                    {extraActions}
                    <Button type="submit" variant={destructive ? "danger" : "primary"} disabled={busy}>
                        {submitLabel}
                    </Button>
                </div>
            </form>
        </Modal>
    );
}
