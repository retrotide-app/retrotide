import type { AdminStatsResponse } from "../../types/api";
import { apiFetch } from "../client";

export async function getAdminStats(): Promise<AdminStatsResponse> {
    return apiFetch<AdminStatsResponse>("/admin/stats");
}
