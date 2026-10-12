import { expect, it, vi, type Mock } from "vitest";
import { apiDelete, apiDeleteWithBody, apiFetch, apiPatch, apiPost, apiPut } from "../client";

export interface RequestCase {
    name: string;
    call: () => Promise<unknown>;
    transport: Mock;
    request: unknown[];
    strict?: boolean;
}

export const fetchMock = vi.mocked(apiFetch);
export const postMock = vi.mocked(apiPost);
export const putMock = vi.mocked(apiPut);
export const patchMock = vi.mocked(apiPatch);
export const deleteMock = vi.mocked(apiDelete);
export const deleteWithBodyMock = vi.mocked(apiDeleteWithBody);

export function resetTransports(): void {
    fetchMock.mockResolvedValue({});
    postMock.mockResolvedValue({});
    putMock.mockResolvedValue({});
    patchMock.mockResolvedValue({});
    deleteMock.mockResolvedValue({});
    deleteWithBodyMock.mockResolvedValue({});
}

export function runRequestCases(cases: RequestCase[]): void {
    it.each(cases)("$name", async row => {
        // given
        const { call, transport, request, strict } = row;

        // when
        await call();

        // then
        if (strict) {
            expect(transport.mock.lastCall).toStrictEqual(request);
            return;
        }

        expect(transport).toHaveBeenCalledWith(...request);
    });
}
