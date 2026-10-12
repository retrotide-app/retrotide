import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, type Mock, vi } from "vitest";
import { queryKeys } from "../../api/queryKeys";
import { makeApiError, makeBoardCommandResponse, makeBoardSnapshot } from "../../test-utils/fixtures";
import { expectInvalidated } from "../../test-utils/query";
import { createTestQueryClient, providerWrapper } from "../../test-utils/render";
import { useSetBoardBackground, useSetCardImage, useUploadMedia } from "./media";

const endpoints = vi.hoisted(() => ({
    uploadMedia: vi.fn(),
    setCardImage: vi.fn(),
    setBoardBackground: vi.fn(),
}));

vi.mock("../../api/endpoints/media", () => endpoints);

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

const anyId = expect.stringMatching(V4);

const BOARD = "b1";

const BOARD_STATE = queryKeys.boardLive.state(BOARD);

const UPLOADED = { id: "m1", path: "/uploads/b1/m1.webp", width: 640, height: 480 };

interface CommandCase {
    name: string;
    use: (boardId: string) => { mutateAsync: (variables: never) => Promise<unknown> };
    variables: unknown;
    endpoint: Mock;
    request: unknown[];
}

const commands: CommandCase[] = [
    {
        name: "useSetCardImage puts the image on the card",
        use: useSetCardImage,
        variables: { cardId: "c1", imageId: "m1" },
        endpoint: endpoints.setCardImage,
        request: [BOARD, "c1", { op_id: anyId, image_id: "m1" }],
    },
    {
        name: "useSetCardImage takes the image off the card",
        use: useSetCardImage,
        variables: { cardId: "c1", imageId: null },
        endpoint: endpoints.setCardImage,
        request: [BOARD, "c1", { op_id: anyId, image_id: null }],
    },
    {
        name: "useSetBoardBackground puts the background on the board",
        use: useSetBoardBackground,
        variables: { imageId: "m2" },
        endpoint: endpoints.setBoardBackground,
        request: [BOARD, { op_id: anyId, image_id: "m2" }],
    },
];

function renderCommand<T>(use: (boardId: string) => T) {
    const qc = createTestQueryClient();
    const invalidate = vi.spyOn(qc, "invalidateQueries");
    const rendered = renderHook(() => use(BOARD), { wrapper: providerWrapper({ queryClient: qc }) });

    return { ...rendered, qc, invalidate };
}

beforeEach(() => {
    for (const fn of Object.values(endpoints)) {
        fn.mockReset();
    }

    endpoints.uploadMedia.mockResolvedValue(UPLOADED);
    endpoints.setCardImage.mockResolvedValue(makeBoardCommandResponse());
    endpoints.setBoardBackground.mockResolvedValue(makeBoardCommandResponse());
});

describe("useUploadMedia", () => {
    it("uploads the file under its kind and answers with the stored image", async () => {
        // given
        const { result } = renderCommand(useUploadMedia);
        const file = new File(["png-bytes"], "cat.png", { type: "image/png" });

        // when
        let uploaded: unknown;
        await act(async () => {
            uploaded = await result.current.mutateAsync({ kind: "card", file });
        });

        // then
        expect(endpoints.uploadMedia).toHaveBeenCalledWith(BOARD, "card", file);
        expect(uploaded).toEqual(UPLOADED);
    });

    it("leaves the board alone when the upload is refused", async () => {
        // given
        endpoints.uploadMedia.mockRejectedValue(
            makeApiError({
                status: 400,
                message: "only PNG, JPEG and WebP images can be uploaded",
                code: "unsupported_image",
            }),
        );
        const { result, invalidate } = renderCommand(useUploadMedia);

        // when
        await act(async () => {
            await expect(
                result.current.mutateAsync({ kind: "card", file: new File(["gif"], "cat.gif", { type: "image/gif" }) }),
            ).rejects.toThrow("only PNG, JPEG and WebP images can be uploaded");
        });

        // then
        expect(invalidate).not.toHaveBeenCalled();
    });
});

describe("image commands", () => {
    it.each(commands)(
        "$name, and leaves the board for its own frame",
        async ({ use, variables, endpoint, request }) => {
            // given
            const { result, qc, invalidate } = renderCommand(use);
            const held = makeBoardSnapshot({ board_id: BOARD, version: 3 });
            qc.setQueryDefaults(BOARD_STATE, { gcTime: Infinity });
            qc.setQueryData(BOARD_STATE, held);

            // when
            await act(async () => {
                await result.current.mutateAsync(variables as never);
            });

            // then
            expect(endpoint.mock.calls[0]).toEqual(request);
            expect(qc.getQueryData(BOARD_STATE)).toBe(held);
            expect(invalidate).not.toHaveBeenCalled();
        },
    );

    it.each(commands)("files the command under the board's mutation key: $name", async ({ use, variables }) => {
        // given
        const { result, qc } = renderCommand(use);

        // when
        await act(async () => {
            await result.current.mutateAsync(variables as never);
        });

        // then
        const filed = qc.getMutationCache().findAll({ mutationKey: queryKeys.boardLive.mutations(BOARD) });
        expect(filed).toHaveLength(1);
    });

    it.each([403, 404, 409])("refetches the board after a %i", async status => {
        // given
        endpoints.setCardImage.mockRejectedValue(makeApiError({ status, message: "image not found" }));
        const { result, qc } = renderCommand(useSetCardImage);

        // when
        await act(async () => {
            await expect(result.current.mutateAsync({ cardId: "c1", imageId: "m1" })).rejects.toThrow(
                "image not found",
            );
        });

        // then
        expectInvalidated(qc, BOARD_STATE);
    });
});
