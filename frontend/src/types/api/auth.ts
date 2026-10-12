import type { StaffRole } from "./admin";
import type { UserResponse } from "./user";

export interface RegisterRequest {
    email: string;
    password: string;
    display_name: string;
    turnstile_token?: string;
}

export interface RegistrationResponse {
    email: string;
    verification_required: boolean;
}

export interface LoginRequest {
    email: string;
    password: string;
    remember: boolean;
    turnstile_token?: string;
}

export interface ForgotPasswordRequest {
    email: string;
    turnstile_token?: string;
}

export interface ResendVerificationRequest {
    email: string;
    turnstile_token?: string;
}

export interface ResetPasswordRequest {
    token: string;
    new_password: string;
}

export interface VerifyEmailRequest {
    token: string;
}

export interface SetEmailRequest {
    email: string;
    password: string;
}

export interface ChangePasswordRequest {
    current_password: string;
    new_password: string;
}

export interface DeleteAccountRequest {
    password: string;
}

export interface UpdateDisplayNameRequest {
    display_name: string;
}

export interface InvitationTokenRequest {
    token: string;
}

export interface InvitationResponse {
    email: string;
    expires_at: string;
}

export interface AcceptInvitationRequest {
    token: string;
    display_name: string;
    password: string;
}

export interface InvitationAcceptedResponse {
    email: string;
}

export interface SessionGuest {
    id: string;
}

export interface SessionResponse {
    authenticated: boolean;
    user?: UserResponse;
    permissions?: string[];
    site_role?: StaffRole;
    guest?: SessionGuest;
}
