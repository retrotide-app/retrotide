import type {
    TwoFactorCodeRequest,
    TwoFactorEnrolmentRequest,
    TwoFactorEnrolmentResponse,
    TwoFactorRecoveryCodesResponse,
    TwoFactorStatusResponse,
    TwoFactorVerifiedResponse,
    TwoFactorVerifyRequest,
} from "../../types/api";
import { apiDeleteWithBody, apiFetch, apiPost } from "../client";

export async function getTwoFactorStatus(): Promise<TwoFactorStatusResponse> {
    return apiFetch<TwoFactorStatusResponse>("/auth/two-factor");
}

export async function beginTwoFactorEnrolment(body: TwoFactorEnrolmentRequest): Promise<TwoFactorEnrolmentResponse> {
    return apiPost<TwoFactorEnrolmentResponse, TwoFactorEnrolmentRequest>("/auth/two-factor/enrolment", body);
}

export async function confirmTwoFactorEnrolment(body: TwoFactorCodeRequest): Promise<TwoFactorRecoveryCodesResponse> {
    return apiPost<TwoFactorRecoveryCodesResponse, TwoFactorCodeRequest>("/auth/two-factor/confirmation", body);
}

export async function disableTwoFactor(body: TwoFactorCodeRequest): Promise<void> {
    return apiDeleteWithBody<void, TwoFactorCodeRequest>("/auth/two-factor", body);
}

export async function verifyTwoFactor(body: TwoFactorVerifyRequest): Promise<TwoFactorVerifiedResponse> {
    return apiPost<TwoFactorVerifiedResponse, TwoFactorVerifyRequest>("/auth/two-factor/verify", body);
}
