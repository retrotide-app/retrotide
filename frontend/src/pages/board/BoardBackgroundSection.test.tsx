import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeBoardSnapshot, makeBoardView, makeMutationState } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { BoardBackgroundSection } from "./BoardBackgroundSection";

const { useUploadMedia, useSetBoardBackground } = vi.hoisted(() => ({
    useUploadMedia: vi.fn(),
    useSetBoardBackground: vi.fn(),
}));

vi.mock("../../hooks/mutations/media", () => ({ useUploadMedia, useSetBoardBackground }));

const BOARD = "00000000-0000-4000-8000-0000000000b1";
const UPLOADED = { id: "m2", path: `/uploads/${BOARD}/m2.webp`, width: 2560, height: 1440 };

let upload = makeMutationState();
let setBackground = makeMutationState();

function renderSection(backgroundPath?: string) {
    const snapshot = makeBoardSnapshot({ board_id: BOARD, board: makeBoardView({ background_path: backgroundPath }) });

    renderWithProviders(<BoardBackgroundSection snapshot={snapshot} />);
}

function jpegFile(): File {
    return new File(["jpeg-bytes"], "beach.jpg", { type: "image/jpeg" });
}

beforeEach(() => {
    vi.clearAllMocks();
    upload = makeMutationState();
    setBackground = makeMutationState();
    useUploadMedia.mockReturnValue(upload);
    useSetBoardBackground.mockReturnValue(setBackground);
});

describe("BoardBackgroundSection", () => {
    it("says the board has no background yet and offers nothing to remove", () => {
        // when
        renderSection();

        // then
        expect(screen.getByRole("heading", { name: "Background" })).toBeInTheDocument();
        expect(screen.getByText("No background yet.")).toBeInTheDocument();
        expect(screen.getByLabelText("Background image")).toHaveAttribute("accept", "image/png,image/jpeg,image/webp");
        expect(screen.queryByRole("button", { name: "Remove background" })).not.toBeInTheDocument();
    });

    it("uploads the picked file as a background and sets it on the board", async () => {
        // given
        const user = userEvent.setup();
        upload.mutate.mockImplementation((_variables, options) => options.onSuccess(UPLOADED));
        const file = jpegFile();
        renderSection();

        // when
        await user.upload(screen.getByLabelText("Background image"), file);
        await waitFor(() => {
            expect(screen.getByRole("img", { name: "Preview of the board's background" })).toBeInTheDocument();
        });
        await user.click(screen.getByRole("button", { name: "Upload background" }));

        // then
        expect(useUploadMedia).toHaveBeenCalledWith(BOARD);
        expect(useSetBoardBackground).toHaveBeenCalledWith(BOARD);
        expect(upload.mutate).toHaveBeenCalledExactlyOnceWith({ kind: "background", file }, expect.anything());
        expect(setBackground.mutate).toHaveBeenCalledExactlyOnceWith({ imageId: "m2" }, expect.anything());
    });

    it("asks for a file rather than uploading nothing", async () => {
        // given
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(screen.getByRole("button", { name: "Upload background" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Choose an image to upload.");
        expect(upload.mutate).not.toHaveBeenCalled();
    });

    it("shows the current background and removes it", async () => {
        // given
        const user = userEvent.setup();
        renderSection(`/uploads/${BOARD}/m1.webp`);

        // when
        const shown = screen.getByRole("img", { name: "Preview of the board's background" }).getAttribute("src");
        await user.click(screen.getByRole("button", { name: "Remove background" }));

        // then
        expect(shown).toBe(`/uploads/${BOARD}/m1.webp`);
        expect(setBackground.mutate).toHaveBeenCalledExactlyOnceWith({ imageId: null });
        expect(upload.mutate).not.toHaveBeenCalled();
    });

    it("explains an upload over the plan's size limit", () => {
        // given
        upload = makeMutationState({
            status: "error",
            error: makeApiError({ status: 402, message: "upgrade required", code: "upgrade_required" }),
        });
        useUploadMedia.mockReturnValue(upload);

        // when
        renderSection();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("That image is larger than your plan allows.");
    });

    it("holds the controls while the background is on its way", () => {
        // given
        setBackground = makeMutationState({ isPending: true });
        useSetBoardBackground.mockReturnValue(setBackground);

        // when
        renderSection(`/uploads/${BOARD}/m1.webp`);

        // then
        expect(screen.getByLabelText("Background image")).toBeDisabled();
        expect(screen.getByRole("button", { name: "Upload background" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Remove background" })).toBeDisabled();
    });
});
