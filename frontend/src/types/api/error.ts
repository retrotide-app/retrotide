import type { PlanCode, PlanLimit } from "./organisation";

export interface ErrorResponse {
    error: string;
    code?: string;
}

export interface UpgradeRequiredResponse {
    error: string;
    code: string;
    limit: PlanLimit;
    allowed: number;
    current: number;
    plan: PlanCode;
}
