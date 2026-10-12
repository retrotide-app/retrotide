import { afterEach, describe, expect, it, vi } from "vitest";
import { copyText } from "./clipboard";

function defineClipboard(clipboard: Partial<Clipboard> | undefined): void {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: clipboard });
}

afterEach(() => {
    defineClipboard(undefined);
});

describe("copyText", () => {
    it("writes the text to the clipboard", async () => {
        // given
        const writeText = vi.fn().mockResolvedValue(undefined);
        defineClipboard({ writeText });

        // when
        await copyText("https://retrotide.test/invite#token-1");

        // then
        expect(writeText).toHaveBeenCalledExactlyOnceWith("https://retrotide.test/invite#token-1");
    });

    it("passes on the browser's refusal", async () => {
        // given
        const refusal = new DOMException("the document is not focused", "NotAllowedError");
        defineClipboard({ writeText: vi.fn().mockRejectedValue(refusal) });

        // when
        const copying = copyText("text");

        // then
        await expect(copying).rejects.toBe(refusal);
    });

    it("says so when the browser offers no clipboard", async () => {
        // given
        defineClipboard(undefined);

        // when
        const copying = copyText("text");

        // then
        await expect(copying).rejects.toThrow("This browser does not let the page copy text.");
    });
});
