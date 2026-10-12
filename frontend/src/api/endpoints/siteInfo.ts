import type { SiteInfoResponse } from "../../types/api";
import { apiFetch } from "../client";

export async function getSiteInfo(): Promise<SiteInfoResponse> {
    return apiFetch<SiteInfoResponse>("/site-info");
}
