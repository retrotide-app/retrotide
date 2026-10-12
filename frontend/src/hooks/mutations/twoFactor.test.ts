import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { clearSessionLost, isSessionLost, markSessionLost } from "../../api/sessionLost";
import { makeSessionUser } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useBeginTwoFactor, useConfirmTwoFactor, useDisableTwoFactor, useVerifyTwoFactor } from "./twoFactor";

const endpoints = vi.hoisted(() => ({
    getTwoFactorStatus: vi.fn(),
    beginTwoFactorEnrolment: vi.fn(),
    confirmTwoFactorEnrolment: vi.fn(),
    disableTwoFactor: vi.fn(),
    verifyTwoFactor: vi.fn(),
}));

vi.mock("../../api/endpoints/twoFactor", () => endpoints);

const ENROLMENT = {
    secret: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
    otpauth_uri: "otpauth://totp/RetroTide:alice@example.com",
};

const RECOVERY_CODES = ["abcde-fgh23", "jkmnp-qrs45"];

function client() {
    const qc = createTestQueryClient();

    return { qc, invalidate: vi.spyOn(qc, "invalidateQueries") };
}

beforeEach(() => {
    clearSessionLost();
    endpoints.beginTwoFactorEnrolment.mockResolvedValue(ENROLMENT);
    endpoints.confirmTwoFactorEnrolment.mockResolvedValue({ recovery_codes: RECOVERY_CODES });
    endpoints.disableTwoFactor.mockResolvedValue(undefined);
    endpoints.verifyTwoFactor.mockResolvedValue({ user: makeSessionUser() });
});

describe("useBeginTwoFactor", () => {
    it("starts the enrolment and hands back the secret without touching the cache", async () => {
        // given
        const { qc, invalidate } = client();
        const { result } = renderHook(useBeginTwoFactor, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let started: unknown;
        await act(async () => {
            started = await result.current.mutateAsync({ current_password: "harbour-lights" });
        });

        // then
        expect(endpoints.beginTwoFactorEnrolment).toHaveBeenCalledWith(
            { current_password: "harbour-lights" },
            expect.anything(),
        );
        expect(started).toEqual(ENROLMENT);
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("useConfirmTwoFactor", () => {
    it("confirms the code, hands over the recovery codes and reloads the status", async () => {
        // given
        const { qc } = client();
        const onEnabled = vi.fn();
        const { result } = renderHook(() => useConfirmTwoFactor(onEnabled), {
            wrapper: providerWrapper({ queryClient: qc }),
        });

        // when
        await act(async () => {
            await result.current.mutateAsync({ code: "081804" });
        });

        // then
        expect(endpoints.confirmTwoFactorEnrolment).toHaveBeenCalledWith({ code: "081804" }, expect.anything());
        expect(onEnabled).toHaveBeenCalledWith(RECOVERY_CODES);
        expectInvalidated(qc, queryKeys.auth.twoFactor());
    });

    it("hands over nothing when the code is refused", async () => {
        // given
        const { qc, invalidate } = client();
        const onEnabled = vi.fn();
        endpoints.confirmTwoFactorEnrolment.mockRejectedValue(new Error("that code is not right"));
        const { result } = renderHook(() => useConfirmTwoFactor(onEnabled), {
            wrapper: providerWrapper({ queryClient: qc }),
        });

        // when
        await act(async () => {
            await result.current.mutateAsync({ code: "123456" }).catch(() => undefined);
        });

        // then
        expect(onEnabled).not.toHaveBeenCalled();
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("useDisableTwoFactor", () => {
    it("turns two-factor off and reloads the status", async () => {
        // given
        const { qc } = client();
        const { result } = renderHook(useDisableTwoFactor, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync({ code: "081804" });
        });

        // then
        expect(endpoints.disableTwoFactor).toHaveBeenCalledWith({ code: "081804" }, expect.anything());
        expectInvalidated(qc, queryKeys.auth.twoFactor());
    });

    it("leaves the status alone when the code is refused", async () => {
        // given
        const { qc, invalidate } = client();
        endpoints.disableTwoFactor.mockRejectedValue(new Error("that code is not right"));
        const { result } = renderHook(useDisableTwoFactor, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        await act(async () => {
            await result.current.mutateAsync({ code: "123456" }).catch(() => undefined);
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("useVerifyTwoFactor", () => {
    it("finishes the sign-in by clearing a lost session and reloading the session", async () => {
        // given
        markSessionLost();
        const { qc } = client();
        const { result } = renderHook(useVerifyTwoFactor, { wrapper: providerWrapper({ queryClient: qc }) });

        // when
        let verified: unknown;
        await act(async () => {
            verified = await result.current.mutateAsync({ code: "081804", trust_device: false });
        });

        // then
        expect(endpoints.verifyTwoFactor).toHaveBeenCalledWith(
            { code: "081804", trust_device: false },
            expect.anything(),
        );
        expect(verified).toEqual({ user: makeSessionUser() });
        expect(isSessionLost()).toBe(false);
        expectInvalidated(qc, queryKeys.auth.session());
    });
});
