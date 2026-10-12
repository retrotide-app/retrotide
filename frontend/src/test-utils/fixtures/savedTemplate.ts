import type { SavedTemplateResponse } from "../../types/api";

export function makeSavedTemplate(overrides: Partial<SavedTemplateResponse> = {}): SavedTemplateResponse {
    return {
        id: "00000000-0000-4000-8000-0000000000e1",
        scope: "personal",
        name: "Our retro",
        description: "Saved from the sprint review board.",
        columns: [
            { title: "Went well", description: "What should we keep doing?", colour: "green" },
            { title: "To improve", description: "What slowed us down?", colour: "red" },
        ],
        created_at: "2026-10-01T10:00:00Z",
        can_delete: true,
        ...overrides,
    };
}
