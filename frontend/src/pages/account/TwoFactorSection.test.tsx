import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TwoFactorEnrolmentResponse, TwoFactorStatusResponse } from "../../types/api";
import { makeApiError, makeMutationState, makeQueryState, makeSessionUser } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { TwoFactorSection } from "./TwoFactorSection";

const mocks = vi.hoisted(() => ({
    useTwoFactorStatus: vi.fn(),
    useBeginTwoFactor: vi.fn(),
    useConfirmTwoFactor: vi.fn(),
    useDisableTwoFactor: vi.fn(),
    toDataURL: vi.fn(),
    copyText: vi.fn(),
}));

vi.mock("../../hooks/queries/twoFactor", () => ({ useTwoFactorStatus: mocks.useTwoFactorStatus }));
vi.mock("../../hooks/mutations/twoFactor", () => ({
    useBeginTwoFactor: mocks.useBeginTwoFactor,
    useConfirmTwoFactor: mocks.useConfirmTwoFactor,
    useDisableTwoFactor: mocks.useDisableTwoFactor,
}));
vi.mock("qrcode", () => ({ toDataURL: mocks.toDataURL }));
vi.mock("../../platform/clipboard", () => ({ copyText: mocks.copyText }));

const QR_DATA_URL = "data:image/png;base64,iVBORw0KGgo=";

const QR_ALT = "QR code that adds this account to an authenticator app";

const TURN_ON_DIALOG = "Turn on two-factor authentication";

const ENROLMENT_DIALOG = "Add RetroTide to your authenticator app";

const TURN_OFF_DIALOG = "Turn off two-factor authentication";

const PASSWORD_FIRST = "Set a password first, then turn on two-factor authentication.";

const WRONG_CODE = "that code is not right";

const RECOVERY_DIALOG = "Save your recovery codes";

const RECOVERY_CODES = ["abcde-fgh23", "jkmnp-qrs45"];

const ENROLMENT: TwoFactorEnrolmentResponse = {
    secret: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
    otpauth_uri: "otpauth://totp/RetroTide:sam@example.test?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ",
};

const withPassword = makeSessionUser({ has_password: true });
const withoutPassword = makeSessionUser({ has_password: false });

function status(enabled: boolean) {
    return makeQueryState<TwoFactorStatusResponse>({ data: { enabled } });
}

function succeeding(data?: unknown) {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSuccess?: (result: unknown) => void }) => {
            options?.onSuccess?.(data);
        }),
    });
}

function wrongCode() {
    return makeMutationState({
        isError: true,
        error: makeApiError({ status: 400, message: WRONG_CODE, code: "invalid_code" }),
    });
}

function section() {
    return within(screen.getByRole("region", { name: "Two-factor authentication" }));
}

function dialog(name: string) {
    return within(screen.getByRole("dialog", { name }));
}

function renderSection(user = withPassword) {
    renderWithProviders(<TwoFactorSection user={user} />, { user });
}

async function openTurnOn() {
    const user = userEvent.setup();
    await user.click(section().getByRole("button", { name: "Turn on" }));

    return user;
}

async function startEnrolment() {
    const user = await openTurnOn();
    const passwordDialog = dialog(TURN_ON_DIALOG);
    await user.type(passwordDialog.getByLabelText("Current password"), "harbour-lights");
    await user.click(passwordDialog.getByRole("button", { name: "Continue" }));

    return user;
}

beforeEach(() => {
    mocks.useTwoFactorStatus.mockReturnValue(status(false));
    mocks.useBeginTwoFactor.mockReturnValue(makeMutationState());
    mocks.useConfirmTwoFactor.mockReturnValue(makeMutationState());
    mocks.useDisableTwoFactor.mockReturnValue(makeMutationState());
    mocks.toDataURL.mockResolvedValue(QR_DATA_URL);
    mocks.copyText.mockResolvedValue(undefined);
});

function enablesWithRecoveryCodes() {
    mocks.useConfirmTwoFactor.mockImplementation((onEnabled: (recoveryCodes: string[]) => void) =>
        makeMutationState({
            mutate: vi.fn(() => {
                mocks.useTwoFactorStatus.mockReturnValue(status(true));
                onEnabled(RECOVERY_CODES);
            }),
        }),
    );
}

async function turnOnWithCode() {
    const user = await startEnrolment();
    const confirmation = dialog(ENROLMENT_DIALOG);
    await user.type(confirmation.getByLabelText("Code"), "081804");
    await user.click(confirmation.getByRole("button", { name: "Turn on" }));

    return user;
}

describe("TwoFactorSection", () => {
    it("says it is checking while the status loads", () => {
        // given
        mocks.useTwoFactorStatus.mockReturnValue(makeQueryState<TwoFactorStatusResponse>({ loading: true }));

        // when
        renderSection();

        // then
        expect(section().getByText("Checking whether two-factor authentication is on...")).toBeInTheDocument();
        expect(section().queryByRole("button")).not.toBeInTheDocument();
    });

    it("shows a status that could not be read and retries it", async () => {
        // given
        const failed = makeQueryState<TwoFactorStatusResponse>({
            error: makeApiError({ status: 500, message: "failed to read the two-factor status" }),
        });
        mocks.useTwoFactorStatus.mockReturnValue(failed);
        renderSection();

        // when
        await userEvent.setup().click(section().getByRole("button", { name: "Try again" }));

        // then
        expect(section().getByRole("alert")).toHaveTextContent("failed to read the two-factor status");
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("offers to turn it on for an account with a password", () => {
        // when
        renderSection();

        // then
        expect(section().getByText(/^Off\./)).toBeInTheDocument();
        expect(section().getByRole("button", { name: "Turn on" })).toBeInTheDocument();
    });

    it("asks an account without a password to set one first", () => {
        // when
        renderSection(withoutPassword);

        // then
        expect(section().getByText(PASSWORD_FIRST)).toBeInTheDocument();
        expect(section().queryByRole("button", { name: "Turn on" })).not.toBeInTheDocument();
    });

    it("offers to turn it off once it is on", () => {
        // given
        mocks.useTwoFactorStatus.mockReturnValue(status(true));

        // when
        renderSection();

        // then
        expect(section().getByText(/^On\./)).toBeInTheDocument();
        expect(section().getByRole("button", { name: "Turn off" })).toBeInTheDocument();
    });

    describe("turning it on", () => {
        it("asks for the current password before starting", async () => {
            // given
            const begin = makeMutationState();
            mocks.useBeginTwoFactor.mockReturnValue(begin);
            renderSection();
            const user = await openTurnOn();

            // when
            await user.click(dialog(TURN_ON_DIALOG).getByRole("button", { name: "Continue" }));

            // then
            const password = dialog(TURN_ON_DIALOG).getByLabelText("Current password");
            expect(begin.mutate).not.toHaveBeenCalled();
            expect(password).toHaveAccessibleDescription(expect.stringContaining("Enter your current password."));
        });

        it("shows the QR code, the key and a code field once the password is accepted", async () => {
            // given
            const begin = succeeding(ENROLMENT);
            mocks.useBeginTwoFactor.mockReturnValue(begin);
            renderSection();

            // when
            await startEnrolment();

            // then
            const confirmation = dialog(ENROLMENT_DIALOG);
            const qr = await confirmation.findByRole("img", { name: QR_ALT });
            const code = confirmation.getByLabelText("Code");
            expect(begin.mutate).toHaveBeenCalledWith({ current_password: "harbour-lights" }, expect.anything());
            expect(mocks.toDataURL).toHaveBeenCalledWith(ENROLMENT.otpauth_uri, expect.anything());
            expect(qr).toHaveAttribute("src", QR_DATA_URL);
            expect(confirmation.getByText(ENROLMENT.secret)).toBeInTheDocument();
            expect(code).toHaveAttribute("inputmode", "numeric");
            expect(code).toHaveAttribute("autocomplete", "one-time-code");
        });

        it("still shows the key when the QR code cannot be drawn", async () => {
            // given
            mocks.toDataURL.mockRejectedValue(new Error("canvas unavailable"));
            mocks.useBeginTwoFactor.mockReturnValue(succeeding(ENROLMENT));
            renderSection();

            // when
            await startEnrolment();

            // then
            const confirmation = dialog(ENROLMENT_DIALOG);
            expect(await confirmation.findByRole("alert")).toHaveTextContent("canvas unavailable");
            expect(confirmation.getByText(ENROLMENT.secret)).toBeInTheDocument();
        });

        it("ties a wrong password to its field", async () => {
            // given
            const failure = makeApiError({
                status: 403,
                message: "the current password is incorrect",
                code: "incorrect_password",
            });
            mocks.useBeginTwoFactor.mockReturnValue(makeMutationState({ isError: true, error: failure }));
            renderSection();

            // when
            await openTurnOn();

            // then
            const passwordDialog = dialog(TURN_ON_DIALOG);
            expect(passwordDialog.getByRole("alert")).toHaveTextContent("the current password is incorrect");
            expect(passwordDialog.getByLabelText("Current password")).toHaveAccessibleDescription(
                expect.stringContaining("the current password is incorrect"),
            );
        });

        it("asks for a password first when the server says the account has none", async () => {
            // given
            const failure = makeApiError({
                status: 409,
                message: "set a password before you turn on two-factor authentication",
                code: "password_required",
            });
            mocks.useBeginTwoFactor.mockReturnValue(makeMutationState({ isError: true, error: failure }));
            renderSection();

            // when
            await openTurnOn();

            // then
            expect(dialog(TURN_ON_DIALOG).getByRole("alert")).toHaveTextContent(PASSWORD_FIRST);
        });

        it("confirms the code and closes the dialog", async () => {
            // given
            mocks.useBeginTwoFactor.mockReturnValue(succeeding(ENROLMENT));
            const confirm = succeeding();
            mocks.useConfirmTwoFactor.mockReturnValue(confirm);
            renderSection();
            const user = await startEnrolment();
            const confirmation = dialog(ENROLMENT_DIALOG);

            // when
            await user.type(confirmation.getByLabelText("Code"), "081804");
            await user.click(confirmation.getByRole("button", { name: "Turn on" }));

            // then
            expect(confirm.mutate).toHaveBeenCalledWith({ code: "081804" }, expect.anything());
            expect(screen.queryByRole("dialog", { name: ENROLMENT_DIALOG })).not.toBeInTheDocument();
        });

        it("ties a wrong code to the code field", async () => {
            // given
            mocks.useBeginTwoFactor.mockReturnValue(succeeding(ENROLMENT));
            mocks.useConfirmTwoFactor.mockReturnValue(wrongCode());
            renderSection();

            // when
            await startEnrolment();

            // then
            const code = dialog(ENROLMENT_DIALOG).getByLabelText("Code");
            expect(code).toHaveAccessibleDescription(expect.stringContaining(WRONG_CODE));
        });

        it("asks for a code before confirming", async () => {
            // given
            mocks.useBeginTwoFactor.mockReturnValue(succeeding(ENROLMENT));
            const confirm = makeMutationState();
            mocks.useConfirmTwoFactor.mockReturnValue(confirm);
            renderSection();
            const user = await startEnrolment();

            // when
            await user.click(dialog(ENROLMENT_DIALOG).getByRole("button", { name: "Turn on" }));

            // then
            const code = dialog(ENROLMENT_DIALOG).getByLabelText("Code");
            expect(confirm.mutate).not.toHaveBeenCalled();
            expect(code).toHaveAccessibleDescription(expect.stringContaining("Enter the 6-digit code"));
        });

        it("shows the recovery codes once two-factor is on, and keeps them on screen as the status turns on", async () => {
            // given
            mocks.useBeginTwoFactor.mockReturnValue(succeeding(ENROLMENT));
            enablesWithRecoveryCodes();
            renderSection();

            // when
            await turnOnWithCode();

            // then
            const codes = dialog(RECOVERY_DIALOG);
            expect(
                within(codes.getByRole("list", { name: "Recovery codes" }))
                    .getAllByRole("listitem")
                    .map(item => item.textContent),
            ).toEqual(RECOVERY_CODES);
            expect(codes.getByText(/only time these codes are shown/)).toBeInTheDocument();
            expect(codes.getByText(/signed out/)).toBeInTheDocument();
            expect(section().getByText(/^On\./)).toBeInTheDocument();
        });

        it("copies the recovery codes", async () => {
            // given
            mocks.useBeginTwoFactor.mockReturnValue(succeeding(ENROLMENT));
            enablesWithRecoveryCodes();
            renderSection();
            const user = await turnOnWithCode();

            // when
            await user.click(dialog(RECOVERY_DIALOG).getByRole("button", { name: "Copy codes" }));

            // then
            expect(mocks.copyText).toHaveBeenCalledWith(expect.stringContaining("abcde-fgh23\njkmnp-qrs45"));
            expect(await dialog(RECOVERY_DIALOG).findByRole("status")).toHaveTextContent("Codes copied.");
        });

        it("never shows the recovery codes again once they are saved", async () => {
            // given
            mocks.useBeginTwoFactor.mockReturnValue(succeeding(ENROLMENT));
            enablesWithRecoveryCodes();
            renderSection();
            const user = await turnOnWithCode();

            // when
            await user.click(dialog(RECOVERY_DIALOG).getByRole("button", { name: "I've saved my codes" }));

            // then
            expect(screen.queryByRole("dialog", { name: RECOVERY_DIALOG })).not.toBeInTheDocument();
            expect(screen.queryByText("abcde-fgh23")).not.toBeInTheDocument();
            expect(section().queryByRole("button", { name: /recovery codes/i })).not.toBeInTheDocument();
        });
    });

    describe("turning it off", () => {
        beforeEach(() => {
            mocks.useTwoFactorStatus.mockReturnValue(status(true));
        });

        it("turns it off with a current code", async () => {
            // given
            const disable = succeeding();
            mocks.useDisableTwoFactor.mockReturnValue(disable);
            renderSection();
            const user = userEvent.setup();
            await user.click(section().getByRole("button", { name: "Turn off" }));
            const turnOff = dialog(TURN_OFF_DIALOG);

            // when
            await user.type(turnOff.getByLabelText("Code"), "081804");
            await user.click(turnOff.getByRole("button", { name: "Turn off" }));

            // then
            expect(disable.mutate).toHaveBeenCalledWith({ code: "081804" }, expect.anything());
            expect(screen.queryByRole("dialog", { name: TURN_OFF_DIALOG })).not.toBeInTheDocument();
        });

        it("ties a wrong code to the code field", async () => {
            // given
            mocks.useDisableTwoFactor.mockReturnValue(wrongCode());
            renderSection();

            // when
            await userEvent.setup().click(section().getByRole("button", { name: "Turn off" }));

            // then
            const code = dialog(TURN_OFF_DIALOG).getByLabelText("Code");
            expect(code).toHaveAccessibleDescription(expect.stringContaining(WRONG_CODE));
        });

        it("turns it off with a recovery code, typed on a full keyboard", async () => {
            // given someone who has lost their authenticator app
            const disable = succeeding();
            mocks.useDisableTwoFactor.mockReturnValue(disable);
            renderSection();
            const user = userEvent.setup();
            await user.click(section().getByRole("button", { name: "Turn off" }));
            const turnOff = dialog(TURN_OFF_DIALOG);
            const code = turnOff.getByLabelText("Code");

            // when
            await user.type(code, "abcde-fgh23");
            await user.click(turnOff.getByRole("button", { name: "Turn off" }));

            // then
            expect(code).not.toHaveAttribute("inputmode");
            expect(disable.mutate).toHaveBeenCalledWith({ code: "abcde-fgh23" }, expect.anything());
        });
    });
});
