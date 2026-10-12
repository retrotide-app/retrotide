import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError } from "../../test-utils/fixtures";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useTwoFactorStatus } from "./twoFactor";

const endpoints = vi.hoisted(() => ({ getTwoFactorStatus: vi.fn() }));

vi.mock("../../api/endpoints/twoFactor", () => endpoints);

function setup() {
    const queryClient = createTestQueryClient();
    const rendered = renderHook(useTwoFactorStatus, { wrapper: providerWrapper({ queryClient }) });

    return { ...rendered, queryClient };
}

beforeEach(() => {
    endpoints.getTwoFactorStatus.mockResolvedValue({ enabled: false });
});

describe("useTwoFactorStatus", () => {
    it("reads the status under the two-factor key", async () => {
        // given
        const status = { enabled: true };
        endpoints.getTwoFactorStatus.mockResolvedValue(status);

        // when
        const { result, queryClient } = setup();

        // then
        await waitFor(() => expect(result.current.data).toEqual(status));
        expect(queryClient.getQueryData(queryKeys.auth.twoFactor())).toEqual(status);
    });

    it("hands back the error when the status could not be read", async () => {
        // given
        const failure = makeApiError({ status: 500, message: "failed to read the two-factor status" });
        endpoints.getTwoFactorStatus.mockRejectedValue(failure);

        // when
        const { result } = setup();

        // then
        await waitFor(() => expect(result.current.error).toBe(failure));
    });
});
