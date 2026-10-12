import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeTurnstileControl } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { Button } from "../Button/Button";
import { SetPasswordForm } from "./SetPasswordForm";

const { useForgotPassword, useTurnstile } = vi.hoisted(() => ({
    useForgotPassword: vi.fn(),
    useTurnstile: vi.fn(),
}));

vi.mock("../../hooks/mutations/auth", () => ({ useForgotPassword }));
vi.mock("../../hooks/useTurnstile", () => ({ useTurnstile }));

const EMAIL = "sam@gmail.com";

function settling() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSettled?: () => void }) => {
            options?.onSettled?.();
        }),
    });
}

beforeEach(() => {
    useForgotPassword.mockReturnValue(makeMutationState());
    useTurnstile.mockReturnValue(makeTurnstileControl());
});

describe("SetPasswordForm", () => {
    it("sends the reset link to the account's own address", async () => {
        // given
        const request = makeMutationState();
        useForgotPassword.mockReturnValue(request);
        const user = userEvent.setup();
        renderWithProviders(<SetPasswordForm email={EMAIL} />);

        // when
        await user.click(screen.getByRole("button", { name: "Set a password" }));

        // then
        expect(request.mutate).toHaveBeenCalledExactlyOnceWith({ email: EMAIL }, expect.anything());
    });

    it("says where the link went once it is sent", () => {
        // given
        useForgotPassword.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<SetPasswordForm email={EMAIL} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent(
            "Check sam@gmail.com for the link. It works for one hour, and using it signs you out on every device.",
        );
    });

    it("shows the server's reason when the link could not be sent and moves focus to it", () => {
        // given
        const failure = makeApiError({ status: 503, message: "email is not available on this site" });
        useForgotPassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<SetPasswordForm email={EMAIL} />);

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("email is not available on this site");
        expect(alert).toHaveFocus();
    });

    it("says the link could not be sent when the failure carries no message", () => {
        // given
        const failure = makeApiError({ status: 500, message: "" });
        useForgotPassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<SetPasswordForm email={EMAIL} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("The link could not be sent. Try again.");
    });

    it("holds the button while the link is being sent", () => {
        // given
        useForgotPassword.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<SetPasswordForm email={EMAIL} />);

        // then
        expect(screen.getByRole("button", { name: "Set a password" })).toBeDisabled();
    });

    it("draws its button in the variant it is given", () => {
        // given
        const variant = "secondary";

        // when
        renderWithProviders(
            <>
                <SetPasswordForm email={EMAIL} variant={variant} />
                <Button variant={variant}>Reference</Button>
            </>,
        );

        // then
        expect(screen.getByRole("button", { name: "Set a password" }).className).toBe(
            screen.getByRole("button", { name: "Reference" }).className,
        );
    });
});

describe("SetPasswordForm with Turnstile", () => {
    it("draws the verification check while the site has it switched on", () => {
        // given
        const turnstile = makeTurnstileControl({ enabled: true });
        useTurnstile.mockReturnValue(turnstile);

        // when
        renderWithProviders(<SetPasswordForm email={EMAIL} />);

        // then
        expect(turnstile.attach).toHaveBeenCalledWith(expect.any(HTMLDivElement));
    });

    it("asks for the verification check before sending anything", async () => {
        // given
        const request = makeMutationState();
        useForgotPassword.mockReturnValue(request);
        useTurnstile.mockReturnValue(makeTurnstileControl({ enabled: true, token: "" }));
        const user = userEvent.setup();
        renderWithProviders(<SetPasswordForm email={EMAIL} />);

        // when
        await user.click(screen.getByRole("button", { name: "Set a password" }));

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("Complete the verification check.");
        expect(alert).toHaveFocus();
        expect(request.mutate).not.toHaveBeenCalled();
        expect(request.reset).toHaveBeenCalledOnce();
    });

    it("sends the verification token with the request, then spends it", async () => {
        // given
        const request = settling();
        useForgotPassword.mockReturnValue(request);
        const turnstile = makeTurnstileControl({ enabled: true, token: "turnstile-1" });
        useTurnstile.mockReturnValue(turnstile);
        const user = userEvent.setup();
        renderWithProviders(<SetPasswordForm email={EMAIL} />);

        // when
        await user.click(screen.getByRole("button", { name: "Set a password" }));

        // then
        expect(request.mutate).toHaveBeenCalledExactlyOnceWith(
            { email: EMAIL, turnstile_token: "turnstile-1" },
            expect.anything(),
        );
        expect(turnstile.reset).toHaveBeenCalledOnce();
    });
});
