import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeBoardSnapshot, makeCard, makeMutationState } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { CardView } from "../../../types/api";
import { CardImageDialog } from "./CardImageDialog";

const { useUploadMedia, useSetCardImage } = vi.hoisted(() => ({ useUploadMedia: vi.fn(), useSetCardImage: vi.fn() }));

vi.mock("../../../hooks/mutations/media", () => ({ useUploadMedia, useSetCardImage }));

const BOARD = "00000000-0000-4000-8000-0000000000b1";
const CARD = "00000000-0000-4000-8000-000000000a01";
const UPLOADED = { id: "m1", path: `/uploads/${BOARD}/m1.webp`, width: 640, height: 480 };

let upload = makeMutationState();
let setImage = makeMutationState();

function renderDialog(card: CardView = makeCard({ id: CARD }), onClose = vi.fn()) {
    renderWithProviders(
        <CardImageDialog snapshot={makeBoardSnapshot({ board_id: BOARD })} card={card} onClose={onClose} />,
    );

    return onClose;
}

function pngFile(): File {
    return new File(["png-bytes"], "cat.png", { type: "image/png" });
}

beforeEach(() => {
    vi.clearAllMocks();
    upload = makeMutationState();
    setImage = makeMutationState();
    useUploadMedia.mockReturnValue(upload);
    useSetCardImage.mockReturnValue(setImage);
});

describe("CardImageDialog", () => {
    it("offers only the image formats the server takes", () => {
        // when
        renderDialog();

        // then
        expect(screen.getByRole("dialog", { name: "Add an image to the card" })).toBeInTheDocument();
        expect(screen.getByLabelText("Image file")).toHaveAttribute("accept", "image/png,image/jpeg,image/webp");
        expect(screen.getByText("No image yet.")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Remove image" })).not.toBeInTheDocument();
    });

    it("previews the picked file before it is uploaded", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.upload(screen.getByLabelText("Image file"), pngFile());

        // then
        await waitFor(() => {
            expect(screen.getByRole("img", { name: "Preview of the card's image" })).toHaveAttribute(
                "src",
                `data:image/png;base64,${btoa("png-bytes")}`,
            );
        });
    });

    it("uploads the file as a card image, attaches it and closes", async () => {
        // given
        const user = userEvent.setup();
        upload.mutate.mockImplementation((_variables, options) => options.onSuccess(UPLOADED));
        setImage.mutate.mockImplementation((_variables, options) => options.onSuccess());
        const file = pngFile();
        const onClose = renderDialog();

        // when
        await user.upload(screen.getByLabelText("Image file"), file);
        await user.click(screen.getByRole("button", { name: "Upload image" }));

        // then
        expect(useUploadMedia).toHaveBeenCalledWith(BOARD);
        expect(useSetCardImage).toHaveBeenCalledWith(BOARD);
        expect(upload.mutate).toHaveBeenCalledExactlyOnceWith({ kind: "card", file }, expect.anything());
        expect(setImage.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: CARD, imageId: "m1" }, expect.anything());
        expect(onClose).toHaveBeenCalledOnce();
    });

    it("asks for a file rather than uploading nothing", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Upload image" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Choose an image to upload.");
        expect(upload.mutate).not.toHaveBeenCalled();
    });

    it("shows the card's image and removes it", async () => {
        // given
        const user = userEvent.setup();
        setImage.mutate.mockImplementation((_variables, options) => options.onSuccess());
        const onClose = renderDialog(makeCard({ id: CARD, image_path: `/uploads/${BOARD}/m0.webp` }));

        // when
        const current = screen.getByRole("img", { name: "Preview of the card's image" }).getAttribute("src");
        await user.click(screen.getByRole("button", { name: "Remove image" }));

        // then
        expect(screen.getByRole("dialog", { name: "Change the card's image" })).toBeInTheDocument();
        expect(current).toBe(`/uploads/${BOARD}/m0.webp`);
        expect(setImage.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: CARD, imageId: null }, expect.anything());
        expect(upload.mutate).not.toHaveBeenCalled();
        expect(onClose).toHaveBeenCalledOnce();
    });

    it.each([
        { code: "upgrade_required", status: 402, text: "That image is larger than your plan allows." },
        { code: "unsupported_image", status: 400, text: "Only PNG, JPEG and WebP images can be used." },
        { code: "image_dimensions", status: 400, text: "That image is too large. It can have at most 40 megapixels." },
        { code: "images_unavailable", status: 503, text: "Images cannot be processed right now." },
    ])("explains an upload refused with $code", ({ code, status, text }) => {
        // given
        upload = makeMutationState({ status: "error", error: makeApiError({ status, message: "server words", code }) });
        useUploadMedia.mockReturnValue(upload);

        // when
        renderDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(text);
    });

    it("holds every control while the image is on its way", () => {
        // given
        upload = makeMutationState({ isPending: true });
        useUploadMedia.mockReturnValue(upload);

        // when
        renderDialog(makeCard({ id: CARD, image_path: `/uploads/${BOARD}/m0.webp` }));

        // then
        expect(screen.getByLabelText("Image file")).toBeDisabled();
        expect(screen.getByRole("button", { name: "Upload image" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Remove image" })).toBeDisabled();
    });
});
