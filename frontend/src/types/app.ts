import type { SiteInfoResponse, UserResponse } from "./api";

export type ThemePreference = "system" | "light" | "dark";

export type SessionUser = UserResponse;

export type SiteInfo = SiteInfoResponse;

export interface Viewer {
    kind: "user" | "guest";
    id: string;
}
