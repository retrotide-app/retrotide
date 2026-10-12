import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
    beginTwoFactorEnrolment,
    confirmTwoFactorEnrolment,
    disableTwoFactor,
    verifyTwoFactor,
} from "../../api/endpoints/twoFactor";
import { queryKeys } from "../../api/queryKeys";
import { signedIn } from "./auth";

export function useBeginTwoFactor() {
    return useMutation({ mutationFn: beginTwoFactorEnrolment });
}

export function useConfirmTwoFactor(onEnabled: (recoveryCodes: string[]) => void) {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: confirmTwoFactorEnrolment,
        onSuccess: response => {
            onEnabled(response.recovery_codes);

            return qc.invalidateQueries({ queryKey: queryKeys.auth.twoFactor() });
        },
    });
}

export function useDisableTwoFactor() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: disableTwoFactor,
        onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.auth.twoFactor() }),
    });
}

export function useVerifyTwoFactor() {
    const qc = useQueryClient();

    return useMutation({
        mutationFn: verifyTwoFactor,
        onSuccess: () => signedIn(qc),
    });
}
