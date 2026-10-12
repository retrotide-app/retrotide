import { Button } from "../../../components/Button/Button";
import { FormField } from "../../../components/FormField/FormField";
import { TextField } from "../../../components/FormField/TextField";
import { Input } from "../../../components/Input/Input";
import { Select } from "../../../components/Select/Select";
import { ToggleSwitch } from "../../../components/ToggleSwitch/ToggleSwitch";
import {
    isSecretSet,
    type SiteSettingField,
    settingOptions,
    TOGGLE_OFF,
    TOGGLE_ON,
} from "../../../domain/siteSettings";
import styles from "./AdminSettingsPage.module.css";

const SAVED_SECRET_DOTS = "••••••••••••";

interface SettingControlProps {
    field: SiteSettingField;
    value: string;
    savedValue: string;
    replacing: boolean;
    disabled: boolean;
    onChange: (value: string) => void;
    onReplace: () => void;
    onKeep: () => void;
}

function SecretControl({
    field,
    value,
    savedValue,
    replacing,
    disabled,
    onChange,
    onReplace,
    onKeep,
}: SettingControlProps) {
    if (isSecretSet(savedValue) && !replacing) {
        return (
            <FormField label={field.label} hint={field.hint ?? "Saved. It is never shown again."}>
                {control => (
                    <div className={styles.secretRow}>
                        <Input
                            {...control}
                            fullWidth
                            readOnly
                            value={SAVED_SECRET_DOTS}
                            className={styles.secretValue}
                        />
                        <Button
                            type="button"
                            onClick={onReplace}
                            disabled={disabled}
                            aria-label={`Replace ${field.label}`}
                        >
                            Replace
                        </Button>
                    </div>
                )}
            </FormField>
        );
    }

    const hint = replacing ? "Leave it empty to remove the saved value." : field.hint;

    return (
        <div>
            <TextField
                label={field.label}
                hint={hint}
                type="password"
                autoComplete="new-password"
                spellCheck={false}
                value={value}
                onChange={e => onChange(e.target.value)}
                disabled={disabled}
            />
            {replacing && (
                <div className={styles.secretKeep}>
                    <Button
                        type="button"
                        size="small"
                        variant="ghost"
                        onClick={onKeep}
                        disabled={disabled}
                        aria-label={`Keep saved value for ${field.label}`}
                    >
                        Keep saved value
                    </Button>
                </div>
            )}
        </div>
    );
}

export function SettingControl(props: SettingControlProps) {
    const { field, value, disabled, onChange } = props;

    if (field.kind === "secret") {
        return <SecretControl {...props} />;
    }

    if (field.kind === "toggle") {
        return (
            <ToggleSwitch
                label={field.label}
                description={field.hint}
                enabled={value === TOGGLE_ON}
                onChange={on => onChange(on ? TOGGLE_ON : TOGGLE_OFF)}
                disabled={disabled}
            />
        );
    }

    if (field.kind === "choice") {
        return (
            <FormField label={field.label} hint={field.hint}>
                {control => (
                    <Select {...control} value={value} onChange={e => onChange(e.target.value)} disabled={disabled}>
                        {settingOptions(field, value).map(option => (
                            <option key={option.value} value={option.value}>
                                {option.label}
                            </option>
                        ))}
                    </Select>
                )}
            </FormField>
        );
    }

    return (
        <TextField
            label={field.label}
            hint={field.hint}
            type={field.kind === "number" ? "number" : "text"}
            inputMode={field.kind === "number" ? "numeric" : undefined}
            value={value}
            onChange={e => onChange(e.target.value)}
            disabled={disabled}
        />
    );
}
