import type { BoardLayout } from "../../hooks/board/useBoardLayout";
import { Button } from "../Button/Button";
import styles from "./BoardLayoutToggle.module.css";

interface BoardLayoutToggleProps {
    layout: BoardLayout;
    onChange: (layout: BoardLayout) => void;
}

const LAYOUT_OPTIONS: readonly { layout: BoardLayout; label: string }[] = [
    { layout: "columns", label: "Columns" },
    { layout: "list", label: "List" },
];

export function BoardLayoutToggle({ layout, onChange }: BoardLayoutToggleProps) {
    return (
        <div role="group" aria-label="Board layout" className={styles.toggle}>
            {LAYOUT_OPTIONS.map(option => {
                const current = option.layout === layout;

                return (
                    <Button
                        key={option.layout}
                        type="button"
                        size="small"
                        variant={current ? "primary" : "secondary"}
                        aria-pressed={current}
                        onClick={() => onChange(option.layout)}
                    >
                        {option.label}
                    </Button>
                );
            })}
        </div>
    );
}
