import { toPng } from "html-to-image";
import { afterEach, describe, expect, it, vi } from "vitest";
import { exportNodeAsPng } from "./imageExport";

vi.mock("html-to-image", () => ({ toPng: vi.fn() }));

const toPngMock = vi.mocked(toPng);

afterEach(() => {
    vi.restoreAllMocks();
    toPngMock.mockReset();
});

describe("exportNodeAsPng", () => {
    it("draws the node as a PNG and downloads it under the given name", async () => {
        // given
        const node = document.createElement("div");
        toPngMock.mockResolvedValue("data:image/png;base64,iVBORw0KGgo=");
        const clicked: HTMLAnchorElement[] = [];
        vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function (this: HTMLAnchorElement) {
            clicked.push(this);
        });

        // when
        await exportNodeAsPng(node, "RetroTide - Sprint 12 - 2026-10-12.png");

        // then
        expect(toPngMock).toHaveBeenCalledWith(node);
        expect(clicked).toHaveLength(1);
        expect(clicked[0]?.href).toBe("data:image/png;base64,iVBORw0KGgo=");
        expect(clicked[0]?.download).toBe("RetroTide - Sprint 12 - 2026-10-12.png");
        expect(document.querySelector("a[download]")).toBeNull();
    });

    it("passes on a drawing failure without downloading anything", async () => {
        // given
        toPngMock.mockRejectedValue(new Error("the canvas is tainted"));
        const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);

        // when
        const exported = exportNodeAsPng(document.createElement("div"), "board.png");

        // then
        await expect(exported).rejects.toThrow("the canvas is tainted");
        expect(click).not.toHaveBeenCalled();
    });
});
