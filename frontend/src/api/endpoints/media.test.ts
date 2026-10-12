import { beforeEach, describe, expect, it, vi } from "vitest";
import { apiUpload } from "../client";
import * as api from "./media";
import { putMock, resetTransports, runRequestCases, type RequestCase } from "./testHarness";

vi.mock("../client", async importOriginal => {
    const actual = await importOriginal<typeof import("../client")>();

    return {
        ...actual,
        apiFetch: vi.fn(),
        apiPost: vi.fn(),
        apiPut: vi.fn(),
        apiPatch: vi.fn(),
        apiDelete: vi.fn(),
        apiDeleteWithBody: vi.fn(),
        apiUpload: vi.fn(),
    };
});

const uploadMock = vi.mocked(apiUpload);

beforeEach(() => {
    resetTransports();
    uploadMock.mockResolvedValue({});
});

describe("uploadMedia", () => {
    it("posts the raw file under its kind with the file's own content type", async () => {
        // given
        const file = new File(["png-bytes"], "cat.png", { type: "image/png" });

        // when
        await api.uploadMedia("b/1", "card", file);

        // then
        expect(uploadMock.mock.lastCall).toStrictEqual(["/boards/b%2F1/media?kind=card", file, "image/png"]);
    });

    it("sends a background under its own kind", async () => {
        // given
        const file = new File(["jpeg-bytes"], "beach.jpg", { type: "image/jpeg" });

        // when
        await api.uploadMedia("b1", "background", file);

        // then
        expect(uploadMock.mock.lastCall).toStrictEqual(["/boards/b1/media?kind=background", file, "image/jpeg"]);
    });
});

describe("card images and board backgrounds", () => {
    const cases: RequestCase[] = [
        {
            name: "setCardImage puts the image id on the card with the op id",
            call: () => api.setCardImage("b/1", "c/1", { op_id: "op1", image_id: "m1" }),
            transport: putMock,
            request: ["/boards/b%2F1/cards/c%2F1/image", { op_id: "op1", image_id: "m1" }],
            strict: true,
        },
        {
            name: "setCardImage removes the image with a null id",
            call: () => api.setCardImage("b1", "c1", { op_id: "op2", image_id: null }),
            transport: putMock,
            request: ["/boards/b1/cards/c1/image", { op_id: "op2", image_id: null }],
            strict: true,
        },
        {
            name: "setBoardBackground puts the background image id",
            call: () => api.setBoardBackground("b1", { op_id: "op3", image_id: "m2" }),
            transport: putMock,
            request: ["/boards/b1/background", { op_id: "op3", image_id: "m2" }],
            strict: true,
        },
    ];

    runRequestCases(cases);
});
