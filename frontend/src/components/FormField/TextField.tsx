import type { InputHTMLAttributes, ReactNode } from "react";
import { Input } from "../Input/Input";
import { type FieldLayout, FormField } from "./FormField";

type TextFieldProps = Omit<InputHTMLAttributes<HTMLInputElement>, "id" | "aria-describedby" | "aria-invalid"> & {
    label: string;
    hint?: ReactNode;
    error?: string | null;
    layout?: FieldLayout;
    flush?: boolean;
};

export function TextField({ label, hint, error, layout, flush, ...input }: TextFieldProps) {
    return (
        <FormField label={label} hint={hint} error={error} layout={layout} flush={flush}>
            {control => <Input fullWidth {...input} {...control} />}
        </FormField>
    );
}
