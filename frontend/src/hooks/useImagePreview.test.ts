import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useImagePreview } from "./useImagePreview";

describe("useImagePreview", () => {
    it("has no preview until a file is picked", () => {
        // when
        const { result } = renderHook(() => useImagePreview(null));

        // then
        expect(result.current).toBeNull();
    });

    it("reads the picked file into a data URL the page can show", async () => {
        // given
        const file = new File(["png-bytes"], "cat.png", { type: "image/png" });

        // when
        const { result } = renderHook(() => useImagePreview(file));

        // then
        await waitFor(() => {
            expect(result.current).toBe(`data:image/png;base64,${btoa("png-bytes")}`);
        });
    });

    it("drops the old preview as soon as another file is picked", async () => {
        // given
        const first = new File(["first"], "first.png", { type: "image/png" });
        const second = new File(["second"], "second.png", { type: "image/png" });
        const { result, rerender } = renderHook(({ file }) => useImagePreview(file), { initialProps: { file: first } });
        await waitFor(() => {
            expect(result.current).not.toBeNull();
        });

        // when
        rerender({ file: second });

        // then
        expect(result.current).toBeNull();
        await waitFor(() => {
            expect(result.current).toBe(`data:image/png;base64,${btoa("second")}`);
        });
    });

    it("forgets the preview when the file is cleared", async () => {
        // given
        const initialProps: { picked: File | null } = {
            picked: new File(["png-bytes"], "cat.png", { type: "image/png" }),
        };
        const { result, rerender } = renderHook(({ picked }: { picked: File | null }) => useImagePreview(picked), {
            initialProps,
        });
        await waitFor(() => {
            expect(result.current).not.toBeNull();
        });

        // when
        rerender({ picked: null });

        // then
        expect(result.current).toBeNull();
    });
});
