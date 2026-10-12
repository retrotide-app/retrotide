import { SHORTCUTS } from "../../domain/board/shortcuts";
import { Modal } from "../Modal/Modal";
import styles from "./ShortcutsDialog.module.css";

interface ShortcutsDialogProps {
    open: boolean;
    onClose: () => void;
}

export function ShortcutsDialog({ open, onClose }: ShortcutsDialogProps) {
    return (
        <Modal isOpen={open} onClose={onClose} title="Keyboard shortcuts">
            <dl className={styles.list}>
                {SHORTCUTS.map(shortcut => (
                    <div key={shortcut.keys} className={styles.row}>
                        <dt className={styles.keys}>
                            <kbd>{shortcut.keys}</kbd>
                        </dt>
                        <dd className={styles.description}>{shortcut.description}</dd>
                    </div>
                ))}
            </dl>
            <p className={styles.note}>Shortcuts wait while you type in a field, and while a card is presented.</p>
        </Modal>
    );
}
