import { matchQuery, type QueryClient, useMutation, useQueryClient } from "@tanstack/react-query";
import {
    acceptInvitation,
    changePassword,
    deleteAccount,
    forgotPassword,
    login,
    logout,
    register,
    resendVerification,
    resetPassword,
    setEmail,
    signOutOtherDevices,
    updateProfile,
    verifyEmail,
} from "../../api/endpoints/auth";
import { queryKeys } from "../../api/queryKeys";
import { clearSessionLost } from "../../api/sessionLost";
import { isTwoFactorChallenge } from "../../domain/twoFactor";
import type { SessionResponse, UserResponse } from "../../types/api";

const SIGNED_OUT: SessionResponse = { authenticated: false };

function forgetAccountData(qc: QueryClient): void {
    const kept = [queryKeys.siteInfo(), queryKeys.auth.session()];

    qc.removeQueries({ predicate: query => !kept.some(queryKey => matchQuery({ queryKey }, query)) });
}

export async function signedIn(qc: QueryClient): Promise<void> {
    clearSessionLost();
    forgetAccountData(qc);

    await qc.invalidateQueries({ queryKey: queryKeys.auth.session() });
}

function signedOut(qc: QueryClient): void {
    qc.setQueryData(queryKeys.auth.session(), SIGNED_OUT);
    forgetAccountData(qc);
}

function withUser(session: SessionResponse | undefined, user: UserResponse): SessionResponse | undefined {
    if (session === undefined || !session.authenticated) {
        return session;
    }

    return { ...session, user };
}

export function useRegister() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: register,
        onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.siteInfo() }),
    });
}

export function useLogin() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: login,
        onSuccess: async response => {
            if (isTwoFactorChallenge(response)) {
                return;
            }

            await signedIn(qc);
        },
    });
}

export function useLogout() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: logout,
        onSuccess: () => {
            signedOut(qc);
        },
    });
}

export function useVerifyEmail() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: verifyEmail,
        onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.auth.session() }),
    });
}

export function useResendVerification() {
    return useMutation({ mutationFn: resendVerification });
}

export function useForgotPassword() {
    return useMutation({ mutationFn: forgotPassword });
}

export function useResetPassword() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: resetPassword,
        onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.auth.session() }),
    });
}

export function useAcceptInvitation() {
    return useMutation({ mutationFn: acceptInvitation });
}

export function useSetEmail() {
    return useMutation({ mutationFn: setEmail });
}

export function useChangePassword() {
    return useMutation({ mutationFn: changePassword });
}

export function useUpdateProfile() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: updateProfile,
        onSuccess: user => {
            qc.setQueryData<SessionResponse>(queryKeys.auth.session(), session => withUser(session, user));
        },
    });
}

export function useSignOutOtherDevices() {
    return useMutation({ mutationFn: signOutOtherDevices });
}

export function useDeleteAccount() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: deleteAccount,
        onSuccess: () => {
            signedOut(qc);
        },
    });
}
