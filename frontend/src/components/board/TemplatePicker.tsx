import { useId } from "react";
import type { TemplateOption, TemplateOptionGroup } from "../../domain/savedTemplates";
import colours from "./boardColours.module.css";
import styles from "./TemplatePicker.module.css";

interface TemplatePickerProps {
    legend: string;
    groups: readonly TemplateOptionGroup[];
    selected: string;
    onSelect: (value: string) => void;
    error?: string | null;
}

interface TemplateChoiceProps {
    name: string;
    option: TemplateOption;
    checked: boolean;
    onSelect: (value: string) => void;
}

function TemplateChoice({ name, option, checked, onSelect }: TemplateChoiceProps) {
    const id = useId();
    const nameId = `${id}-name`;
    const noteId = `${id}-note`;
    const descriptionId = `${id}-description`;
    const columnsId = `${id}-columns`;
    const describedBy = [option.note === null ? null : noteId, descriptionId, columnsId]
        .filter(each => each !== null)
        .join(" ");

    return (
        <label className={styles.option}>
            <input
                type="radio"
                name={name}
                value={option.value}
                checked={checked}
                onChange={() => onSelect(option.value)}
                aria-labelledby={nameId}
                aria-describedby={describedBy}
                className={styles.radio}
            />
            <span id={nameId} className={styles.name} dir="auto">
                {option.name}
            </span>
            {option.note !== null && (
                <span id={noteId} className={styles.description} dir="auto">
                    {option.note}
                </span>
            )}
            <span id={descriptionId} className={styles.description} dir="auto">
                {option.description}
            </span>
            <span id={columnsId} className="visually-hidden">
                {`Columns: ${option.columns.map(column => column.title).join(", ")}.`}
            </span>
            <span className={styles.board} aria-hidden="true">
                {option.columns.map((column, index) => (
                    <span key={`${index}:${column.title}`} className={`${styles.column} ${colours[column.colour]}`}>
                        <span className={styles.bar} />
                        <span className={styles.columnTitle} dir="auto">
                            {column.title}
                        </span>
                    </span>
                ))}
            </span>
        </label>
    );
}

export function TemplatePicker({ legend, groups, selected, onSelect, error }: TemplatePickerProps) {
    const id = useId();
    const errorId = `${id}-error`;
    const hasError = error !== undefined && error !== null && error !== "";
    const shown = groups.filter(group => group.options.length > 0);
    const labelled = shown.length > 1;

    return (
        <fieldset
            className={styles.picker}
            aria-describedby={hasError ? errorId : undefined}
            aria-invalid={hasError ? true : undefined}
        >
            <legend className="visually-hidden">{legend}</legend>
            {hasError && (
                <p id={errorId} className={styles.error} dir="auto">
                    {error}
                </p>
            )}
            <div className={styles.groups}>
                {shown.map((group, index) => {
                    const labelId = `${id}-group-${index}`;

                    return (
                        <div
                            key={group.label}
                            role={labelled ? "group" : undefined}
                            aria-labelledby={labelled ? labelId : undefined}
                            className={styles.group}
                        >
                            {labelled && (
                                <p id={labelId} className={styles.groupLabel}>
                                    {group.label}
                                </p>
                            )}
                            <div className={styles.grid}>
                                {group.options.map(option => (
                                    <TemplateChoice
                                        key={option.value}
                                        name={id}
                                        option={option}
                                        checked={option.value === selected}
                                        onSelect={onSelect}
                                    />
                                ))}
                            </div>
                        </div>
                    );
                })}
            </div>
        </fieldset>
    );
}
