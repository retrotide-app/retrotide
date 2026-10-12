import type { TemplateResponse } from "../types/api";
import { errorStatus } from "../utils/errorMessage";
import { newBoardPath } from "./board/paths";
import { TEMPLATES_PATH } from "./marketing";

const BLANK_TEMPLATE_KEY = "blank";

const FRONT_DOOR_PATH = "/";

const NOT_FOUND_STATUS = 404;

const REGISTER_PATH = "/register";

export function isPublicTemplate(key: string): boolean {
    return key !== BLANK_TEMPLATE_KEY;
}

export function publicTemplates(templates: readonly TemplateResponse[]): TemplateResponse[] {
    return templates.filter(template => isPublicTemplate(template.key));
}

export function templatePath(key: string): string {
    return `${TEMPLATES_PATH}/${encodeURIComponent(key)}`;
}

export function isTemplateNotFound(error: unknown): boolean {
    return errorStatus(error) === NOT_FOUND_STATUS;
}

export function columnCountLabel(count: number): string {
    return count === 1 ? "1 column" : `${count} columns`;
}

export function templateStartPath(signedIn: boolean, orgSlug: string | null): string {
    if (!signedIn) {
        return REGISTER_PATH;
    }

    if (orgSlug === null) {
        return FRONT_DOOR_PATH;
    }

    return newBoardPath(orgSlug);
}
