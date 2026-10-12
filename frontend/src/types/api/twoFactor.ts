import type { UserResponse } from "./user";

export interface TwoFactorStatusResponse {
    enabled: boolean;
}

export interface TwoFactorEnrolmentRequest {
    current_password: string;
}

export interface TwoFactorEnrolmentResponse {
    secret: string;
    otpauth_uri: string;
}

export interface TwoFactorCodeRequest {
    code: string;
}

export interface TwoFactorVerifyRequest {
    code: string;
    trust_device: boolean;
}

export interface TwoFactorRecoveryCodesResponse {
    recovery_codes: string[];
}

export interface TwoFactorChallengeResponse {
    two_factor_required: true;
}

export interface TwoFactorVerifiedResponse {
    user: UserResponse;
    location?: string;
}
