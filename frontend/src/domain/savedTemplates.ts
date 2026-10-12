import type {
    CreateBoardRequest,
    SavedTemplateResponse,
    SavedTemplateScope,
    TemplateColumnResponse,
    TemplateResponse,
} from "../types/api";
import { errorCode } from "../utils/errorMessage";
import { newBoardPath } from "./board/paths";
import { organisationHomePath } from "./organisations";

export type TemplateChoice = { kind: "builtin"; key: string } | { kind: "saved"; id: string };

export interface TemplateOption {
    value: string;
    choice: TemplateChoice;
    name: string;
    description: string;
    note: string | null;
    columns: readonly TemplateColumnResponse[];
}

export interface TemplateOptionGroup {
    label: string;
    options: readonly TemplateOption[];
}

export type SavedTemplateField = "name" | "description" | "scope";

export const SAVED_TEMPLATE_PARAM = "saved";

export const SAVED_TEMPLATE_NAME_MAX_LENGTH = 80;

export const SAVED_TEMPLATE_DESCRIPTION_MAX_LENGTH = 300;

export const SAVED_TEMPLATES_GROUP_LABEL = "Your templates";

export const BUILT_IN_TEMPLATES_GROUP_LABEL = "Built-in templates";

export const SAVED_TEMPLATE_SCOPE_LABELS: Readonly<Record<SavedTemplateScope, string>> = {
    personal: "Only me",
    team: "Everyone on the board's team",
};

const BUILT_IN_PREFIX = "builtin:";

const SAVED_PREFIX = "saved:";

const PERSONAL_NOTE = "Only you";

const FIELD_REFUSALS: Readonly<Record<SavedTemplateField, readonly string[]>> = {
    name: ["template_name_required", "template_name_too_long"],
    description: ["template_description_too_long"],
    scope: ["invalid_template_scope", "not_team_board"],
};

export function organisationTemplatesPath(slug: string): string {
    return `${organisationHomePath(slug)}/templates`;
}

export function newBoardFromTemplatePath(slug: string, templateId: string): string {
    const query = new URLSearchParams({ [SAVED_TEMPLATE_PARAM]: templateId });

    return `${newBoardPath(slug)}?${query.toString()}`;
}

export function savedTemplateValue(templateId: string): string {
    return `${SAVED_PREFIX}${templateId}`;
}

export function savedTemplateNote(template: SavedTemplateResponse): string {
    if (template.scope === "personal") {
        return PERSONAL_NOTE;
    }

    if (template.team_name === undefined || template.team_name === "") {
        return "Shared with a team";
    }

    return `Shared with ${template.team_name}`;
}

export function builtInOption(template: TemplateResponse): TemplateOption {
    return {
        value: `${BUILT_IN_PREFIX}${template.key}`,
        choice: { kind: "builtin", key: template.key },
        name: template.name,
        description: template.description,
        note: null,
        columns: template.columns,
    };
}

export function savedOption(template: SavedTemplateResponse): TemplateOption {
    return {
        value: savedTemplateValue(template.id),
        choice: { kind: "saved", id: template.id },
        name: template.name,
        description: template.description,
        note: savedTemplateNote(template),
        columns: template.columns,
    };
}

export function templateGroups(
    saved: readonly SavedTemplateResponse[],
    builtIn: readonly TemplateResponse[],
): TemplateOptionGroup[] {
    const groups: TemplateOptionGroup[] = [];

    if (saved.length > 0) {
        groups.push({ label: SAVED_TEMPLATES_GROUP_LABEL, options: saved.map(savedOption) });
    }

    if (builtIn.length > 0) {
        groups.push({ label: BUILT_IN_TEMPLATES_GROUP_LABEL, options: builtIn.map(builtInOption) });
    }

    return groups;
}

export function pickedTemplateOption(
    groups: readonly TemplateOptionGroup[],
    value: string | null,
): TemplateOption | null {
    const options = groups.flatMap(group => group.options);
    const picked = options.find(option => option.value === value);

    if (picked !== undefined) {
        return picked;
    }

    return options.find(option => option.choice.kind === "builtin") ?? options[0] ?? null;
}

export function templateFields(choice: TemplateChoice): Pick<CreateBoardRequest, "template_key" | "saved_template_id"> {
    if (choice.kind === "saved") {
        return { template_key: "", saved_template_id: choice.id };
    }

    return { template_key: choice.key };
}

export function savedTemplateRefusal(error: Error | null, field: SavedTemplateField): string | null {
    const code = errorCode(error);

    if (error === null || code === null || !FIELD_REFUSALS[field].includes(code)) {
        return null;
    }

    return error.message;
}
