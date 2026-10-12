import type { AdminRecentSignup, AdminStatDay, AdminStatsResponse } from "../../types/api";

const DAY_MS = 86400000;

export function makeAdminStatDays(count = 30, last = "2026-10-05"): AdminStatDay[] {
    const lastDay = Date.parse(`${last}T00:00:00Z`);
    const days: AdminStatDay[] = [];

    for (let index = 0; index < count; index++) {
        const date = new Date(lastDay - (count - 1 - index) * DAY_MS).toISOString().slice(0, 10);
        days.push({ date, users: index % 4, boards: index % 3, cards: (index % 5) * 2 });
    }

    return days;
}

export function makeAdminRecentSignup(overrides: Partial<AdminRecentSignup> = {}): AdminRecentSignup {
    return {
        id: "00000000-0000-4000-8000-0000000000dd",
        display_name: "Morgan",
        email: "morgan@example.test",
        email_verified: true,
        created_at: "2026-10-04T09:30:00Z",
        ...overrides,
    };
}

export function makeAdminStats(overrides: Partial<AdminStatsResponse> = {}): AdminStatsResponse {
    return {
        generated_at: "2026-10-05T12:00:00Z",
        totals: {
            users: { total: 1284, last_7_days: 12 },
            verified_users: { total: 1101, last_7_days: 9 },
            organisations: { total: 214, last_7_days: 3 },
            teams: { total: 530, last_7_days: 0 },
            boards: { total: 2960, last_7_days: 41 },
            cards: { total: 48210, last_7_days: 902 },
        },
        daily: makeAdminStatDays(),
        activity: {
            last_24_hours: { users: 2, organisations: 1, boards: 6, cards: 120 },
            last_7_days: { users: 12, organisations: 3, boards: 41, cards: 902 },
            last_30_days: { users: 50, organisations: 11, boards: 160, cards: 3400 },
        },
        recent_signups: [makeAdminRecentSignup()],
        health: {
            site_mode: "public",
            maintenance_mode: false,
            email_configured: true,
            email_status: "ready",
            google_sign_in_configured: false,
        },
        ...overrides,
    };
}
