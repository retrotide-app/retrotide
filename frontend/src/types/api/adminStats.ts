import type { SiteMode } from "./siteInfo";

export interface AdminStatTotal {
    total: number;
    last_7_days: number;
}

export interface AdminStatTotals {
    users: AdminStatTotal;
    verified_users: AdminStatTotal;
    organisations: AdminStatTotal;
    teams: AdminStatTotal;
    boards: AdminStatTotal;
    cards: AdminStatTotal;
}

export interface AdminStatDay {
    date: string;
    users: number;
    boards: number;
    cards: number;
}

export interface AdminActivityCounts {
    users: number;
    organisations: number;
    boards: number;
    cards: number;
}

export interface AdminActivity {
    last_24_hours: AdminActivityCounts;
    last_7_days: AdminActivityCounts;
    last_30_days: AdminActivityCounts;
}

export interface AdminRecentSignup {
    id: string;
    display_name: string;
    email: string;
    email_verified: boolean;
    created_at: string;
}

export type AdminEmailStatus = "off" | "ready" | "failing";

export interface AdminSiteHealth {
    site_mode: SiteMode;
    maintenance_mode: boolean;
    email_configured: boolean;
    email_status: AdminEmailStatus;
    google_sign_in_configured: boolean;
}

export interface AdminStatsResponse {
    generated_at: string;
    totals: AdminStatTotals;
    daily: AdminStatDay[];
    activity: AdminActivity;
    recent_signups?: AdminRecentSignup[];
    health: AdminSiteHealth;
}
