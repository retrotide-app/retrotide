import { afterEach, describe, expect, it, vi } from "vitest";
import { printPage } from "./print";

afterEach(() => {
    vi.restoreAllMocks();
});

describe("printPage", () => {
    it("opens the browser's print dialog", () => {
        // given
        const print = vi.spyOn(window, "print").mockImplementation(() => undefined);

        // when
        printPage();

        // then
        expect(print).toHaveBeenCalledOnce();
    });
});
