import type {
    AcceptInvitationRequest,
    ChangePasswordRequest,
    DeleteAccountRequest,
    ForgotPasswordRequest,
    InvitationAcceptedResponse,
    InvitationResponse,
    InvitationTokenRequest,
    LoginRequest,
    RegisterRequest,
    RegistrationResponse,
    ResendVerificationRequest,
    ResetPasswordRequest,
    SessionResponse,
    SetEmailRequest,
    TwoFactorChallengeResponse,
    UpdateDisplayNameRequest,
    UserResponse,
    VerifyEmailRequest,
} from "../../types/api";
import { apiDeleteWithBody, apiFetch, apiPost, apiPut } from "../client";

export async function register(body: RegisterRequest): Promise<RegistrationResponse> {
    return apiPost<RegistrationResponse, RegisterRequest>("/auth/register", body);
}

export async function login(body: LoginRequest): Promise<UserResponse | TwoFactorChallengeResponse> {
    return apiPost<UserResponse | TwoFactorChallengeResponse, LoginRequest>("/auth/login", body);
}

export async function logout(): Promise<void> {
    return apiPost<void>("/auth/logout");
}

export async function getSession(): Promise<SessionResponse> {
    return apiFetch<SessionResponse>("/auth/session");
}

export async function verifyEmail(body: VerifyEmailRequest): Promise<void> {
    return apiPost<void, VerifyEmailRequest>("/auth/verify-email", body);
}

export async function resendVerification(body: ResendVerificationRequest): Promise<void> {
    return apiPost<void, ResendVerificationRequest>("/auth/resend-verification", body);
}

export async function forgotPassword(body: ForgotPasswordRequest): Promise<void> {
    return apiPost<void, ForgotPasswordRequest>("/auth/forgot-password", body);
}

export async function resetPassword(body: ResetPasswordRequest): Promise<void> {
    return apiPost<void, ResetPasswordRequest>("/auth/reset-password", body);
}

export async function getInvitation(body: InvitationTokenRequest): Promise<InvitationResponse> {
    return apiPost<InvitationResponse, InvitationTokenRequest>("/auth/invitation", body);
}

export async function acceptInvitation(body: AcceptInvitationRequest): Promise<InvitationAcceptedResponse> {
    return apiPost<InvitationAcceptedResponse, AcceptInvitationRequest>("/auth/invitation/accept", body);
}

export async function setEmail(body: SetEmailRequest): Promise<void> {
    return apiPost<void, SetEmailRequest>("/auth/set-email", body);
}

export async function changePassword(body: ChangePasswordRequest): Promise<void> {
    return apiPut<void, ChangePasswordRequest>("/auth/password", body);
}

export async function updateProfile(body: UpdateDisplayNameRequest): Promise<UserResponse> {
    return apiPut<UserResponse, UpdateDisplayNameRequest>("/auth/profile", body);
}

export async function signOutOtherDevices(): Promise<void> {
    return apiPost<void>("/auth/sign-out-other-devices");
}

export async function deleteAccount(body: DeleteAccountRequest): Promise<void> {
    return apiDeleteWithBody<void, DeleteAccountRequest>("/auth/account", body);
}
