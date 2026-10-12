import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useCopyText } from "./useCopyText";

const { copyText } = vi.hoisted(() => ({ copyText: vi.fn<(text: string) => Promise<void>>() }));

vi.mock("../platform/clipboard", () => ({ copyText }));

beforeEach(() => {
    copyText.mockReset();
    copyText.mockResolvedValue(undefined);
});

describe("useCopyText", () => {
    it("copies the text and says it did", async () => {
        // given
        const { result } = renderHook(useCopyText);

        // when
        act(() => {
            result.current.copy("https://retrotide.test/invite#token-1");
        });

        // then
        await waitFor(() => expect(result.current.copied).toBe(true));
        expect(copyText).toHaveBeenCalledExactlyOnceWith("https://retrotide.test/invite#token-1");
        expect(result.current.error).toBeNull();
    });

    it("hands back the browser's refusal instead of claiming the text was copied", async () => {
        // given
        const refusal = new Error("the document is not focused");
        copyText.mockRejectedValue(refusal);
        const { result } = renderHook(useCopyText);

        // when
        act(() => {
            result.current.copy("text");
        });

        // then
        await waitFor(() => expect(result.current.error).toBe(refusal));
        expect(result.current.copied).toBe(false);
    });

    it("wraps a refusal that is not an error, keeping it as the cause", async () => {
        // given
        copyText.mockRejectedValue("denied");
        const { result } = renderHook(useCopyText);

        // when
        act(() => {
            result.current.copy("text");
        });

        // then
        await waitFor(() => expect(result.current.error?.message).toBe("denied"));
        expect(result.current.error?.cause).toBe("denied");
    });

    it("forgets the last copy when reset", async () => {
        // given
        const { result } = renderHook(useCopyText);
        act(() => {
            result.current.copy("text");
        });
        await waitFor(() => expect(result.current.copied).toBe(true));

        // when
        act(() => {
            result.current.reset();
        });

        // then
        expect(result.current.copied).toBe(false);
        expect(result.current.error).toBeNull();
    });
});
