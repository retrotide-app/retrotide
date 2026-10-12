import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { QueryState } from "../../hooks/queries/queryState";
import {
    BOARD_ID,
    makeApiError,
    makeMutationState,
    makeQueryState,
    type MutationStateFixture,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSharingResponse } from "../../types/api";
import { ShareDialog } from "./ShareDialog";

const queries = vi.hoisted(() => ({
    useJoinPreview: vi.fn(),
    useBoardSharing: vi.fn(),
}));

const mutations = vi.hoisted(() => ({
    useJoinBoardByLink: vi.fn(),
    useSetLinkAccess: vi.fn(),
    useSetBoardPassword: vi.fn(),
    useRotateShareLink: vi.fn(),
}));

const clipboard = vi.hoisted(() => ({ copyText: vi.fn() }));

const qr = vi.hoisted(() => ({ toDataURL: vi.fn() }));

vi.mock("../../hooks/queries/shareLinks", () => queries);
vi.mock("../../hooks/mutations/shareLinks", () => mutations);
vi.mock("../../platform/clipboard", () => clipboard);
vi.mock("qrcode", () => qr);

const QR_DATA_URL = "data:image/png;base64,iVBORw0KGgo=";

const QR_ALT = "QR code for the board link";

const sharing: BoardSharingResponse = { share_slug: "k7Q2", link_access: "guests", has_password: false };

let access: MutationStateFixture;
let password: MutationStateFixture;
let rotate: MutationStateFixture;

function showSharing(state: Partial<QueryState<BoardSharingResponse>>) {
    const shown = makeQueryState<BoardSharingResponse>(state);
    queries.useBoardSharing.mockReturnValue(shown);

    return shown;
}

function renderDialog(onClose = vi.fn()) {
    renderWithProviders(<ShareDialog boardId={BOARD_ID} onClose={onClose} />);

    return onClose;
}

beforeEach(() => {
    vi.clearAllMocks();
    access = makeMutationState();
    password = makeMutationState();
    rotate = makeMutationState();
    mutations.useSetLinkAccess.mockReturnValue(access);
    mutations.useSetBoardPassword.mockReturnValue(password);
    mutations.useRotateShareLink.mockReturnValue(rotate);
    clipboard.copyText.mockResolvedValue(undefined);
    qr.toDataURL.mockResolvedValue(QR_DATA_URL);
    showSharing({ data: sharing });
});

describe("ShareDialog", () => {
    it("reads the sharing of the board it was opened on", () => {
        // when
        renderDialog();

        // then
        expect(queries.useBoardSharing).toHaveBeenCalledWith(BOARD_ID);
        expect(screen.getByRole("dialog", { name: "Share this board" })).toBeInTheDocument();
    });

    it("shows the full link and copies it", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Copy link" }));

        // then
        expect(screen.getByLabelText("Link to the board")).toHaveValue("http://localhost:3000/b/k7Q2");
        expect(clipboard.copyText).toHaveBeenCalledWith("http://localhost:3000/b/k7Q2");
        expect(await screen.findByText("Link copied.")).toBeInTheDocument();
    });

    it("draws a QR code of the full link", async () => {
        // when
        renderDialog();

        // then
        expect(await screen.findByRole("img", { name: QR_ALT })).toHaveAttribute("src", QR_DATA_URL);
        expect(qr.toDataURL).toHaveBeenCalledWith("http://localhost:3000/b/k7Q2", expect.anything());
    });

    it("says the QR code is being drawn until it is ready", () => {
        // given
        qr.toDataURL.mockReturnValue(new Promise(() => {}));

        // when
        renderDialog();

        // then
        expect(screen.getByText("Drawing the QR code...")).toBeInTheDocument();
        expect(screen.queryByRole("img", { name: QR_ALT })).not.toBeInTheDocument();
    });

    it("says why the QR code could not be drawn, and still offers the link", async () => {
        // given
        qr.toDataURL.mockRejectedValue(new Error("data too long"));

        // when
        renderDialog();

        // then
        expect(await screen.findByRole("alert")).toHaveTextContent(
            "The QR code could not be drawn (data too long). Use the link instead.",
        );
        expect(screen.getByLabelText("Link to the board")).toHaveValue("http://localhost:3000/b/k7Q2");
    });

    it("says why copying failed", async () => {
        // given
        clipboard.copyText.mockRejectedValue(new Error("This browser does not let the page copy text."));
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Copy link" }));

        // then
        expect(await screen.findByRole("alert")).toHaveTextContent("This browser does not let the page copy text.");
    });

    it("marks the current link access and changes it", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();
        const group = screen.getByRole("group", { name: "Who can join with the link" });

        // when
        await user.click(within(group).getByRole("button", { name: "Signed in only" }));

        // then
        expect(within(group).getByRole("button", { name: "Anyone with the link" })).toHaveAttribute(
            "aria-pressed",
            "true",
        );
        expect(access.mutate).toHaveBeenCalledWith("accounts");
    });

    it("does not resend the access the board already has", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();
        const group = screen.getByRole("group", { name: "Who can join with the link" });

        // when
        await user.click(within(group).getByRole("button", { name: "Anyone with the link" }));

        // then
        expect(access.mutate).not.toHaveBeenCalled();
    });

    it("says nobody can join while the link is off", () => {
        // given
        showSharing({ data: { ...sharing, link_access: "off" } });

        // when
        renderDialog();

        // then
        expect(screen.getByText("The link is switched off, so nobody can join with it yet.")).toBeInTheDocument();
    });

    it("shows why changing the access failed", () => {
        // given
        mutations.useSetLinkAccess.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "the board is read only", code: "read_only" }),
            }),
        );

        // when
        renderDialog();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("the board is read only");
    });

    it("sets a password", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.type(screen.getByLabelText("Password"), "tide");
        await user.click(screen.getByRole("button", { name: "Set password" }));

        // then
        expect(password.mutate).toHaveBeenCalledWith("tide", expect.anything());
        expect(screen.queryByRole("button", { name: "Remove password" })).not.toBeInTheDocument();
    });

    it("asks for a password before sending an empty one", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Set password" }));

        // then
        expect(screen.getByText("Enter a password.")).toBeInTheDocument();
        expect(password.mutate).not.toHaveBeenCalled();
    });

    it("changes or removes a password the board already has", async () => {
        // given
        showSharing({ data: { ...sharing, has_password: true } });
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.type(screen.getByLabelText("New password"), "harbour");
        await user.click(screen.getByRole("button", { name: "Change password" }));
        await user.click(screen.getByRole("button", { name: "Remove password" }));

        // then
        expect(password.mutate).toHaveBeenCalledWith("harbour", expect.anything());
        expect(password.mutate).toHaveBeenCalledWith(null);
    });

    it("shows the server's password refusal once and marks the field invalid", () => {
        // given
        mutations.useSetBoardPassword.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({
                    status: 400,
                    message: "the password is longer than 72 bytes",
                    code: "board_password_too_long",
                }),
            }),
        );

        // when
        renderDialog();

        // then
        expect(screen.getAllByText("the password is longer than 72 bytes")).toHaveLength(1);
        expect(screen.getByLabelText("Password")).toHaveAttribute("aria-invalid", "true");
    });

    it("asks before making a new link, and makes it once confirmed", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Make a new link" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("The current link stops working at once");
        expect(rotate.mutate).not.toHaveBeenCalled();

        // when
        await user.click(within(screen.getByRole("alert")).getByRole("button", { name: "Make a new link" }));

        // then
        expect(rotate.mutate).toHaveBeenCalledWith(undefined, expect.anything());
    });

    it("keeps the link when the facilitator thinks again", async () => {
        // given
        const user = userEvent.setup();
        renderDialog();
        await user.click(screen.getByRole("button", { name: "Make a new link" }));

        // when
        await user.click(screen.getByRole("button", { name: "Keep this link" }));

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        expect(rotate.mutate).not.toHaveBeenCalled();
    });

    it("says a new link was made", () => {
        // given
        mutations.useRotateShareLink.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderDialog();

        // then
        expect(screen.getByText("New link made. Share it with the people you want on the board.")).toBeInTheDocument();
    });

    it("says it is loading the sharing", () => {
        // given
        showSharing({ loading: true });

        // when
        renderDialog();

        // then
        expect(screen.getByText("Loading sharing...")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Copy link" })).not.toBeInTheDocument();
    });

    it("shows a sharing that could not be read, with a retry", async () => {
        // given
        const shown = showSharing({
            error: makeApiError({ status: 403, message: "your role on this board does not allow that" }),
        });
        const user = userEvent.setup();
        renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("your role on this board does not allow that");
        expect(shown.refetch).toHaveBeenCalledOnce();
    });

    it("closes with Done", async () => {
        // given
        const user = userEvent.setup();
        const onClose = renderDialog();

        // when
        await user.click(screen.getByRole("button", { name: "Done" }));

        // then
        expect(onClose).toHaveBeenCalled();
    });
});
