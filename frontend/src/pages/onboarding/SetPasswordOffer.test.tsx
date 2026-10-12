import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeTurnstileControl } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { SetPasswordOffer } from "./SetPasswordOffer";

const { useForgotPassword, useTurnstile } = vi.hoisted(() => ({
    useForgotPassword: vi.fn(),
    useTurnstile: vi.fn(),
}));

vi.mock("../../hooks/mutations/auth", () => ({ useForgotPassword }));
vi.mock("../../hooks/useTurnstile", () => ({ useTurnstile }));

const EMAIL = "sam@gmail.com";

function settlingMutation() {
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

describe("SetPasswordOffer", () => {
    it("says the account has no password and where the link goes", () => {
        // given
        const email = EMAIL;

        // when
        renderWithProviders(<SetPasswordOffer email={email} />);

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Set a password" })).toBeInTheDocument();
        expect(screen.getByText(/Your account has no password/)).toHaveTextContent(
            "Your account has no password. We will email a link to sam@gmail.com that lets you set one, so you can still sign in if you lose access to the account you signed up with.",
        );
    });

    it("sends the link to the account's address", async () => {
        // given
        const request = makeMutationState();
        useForgotPassword.mockReturnValue(request);
        const user = userEvent.setup();
        renderWithProviders(<SetPasswordOffer email={EMAIL} />);

        // when
        await user.click(screen.getByRole("button", { name: "Set a password" }));

        // then
        expect(request.mutate).toHaveBeenCalledExactlyOnceWith({ email: EMAIL }, expect.anything());
    });

    it("says where to find the link once it is sent", () => {
        // given
        useForgotPassword.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<SetPasswordOffer email={EMAIL} />);

        // then
        expect(screen.getByRole("status")).toHaveTextContent(
            "Check sam@gmail.com for the link. It works for one hour, and using it signs you out on every device.",
        );
    });

    it("shows why the link could not be sent and moves focus to it", () => {
        // given
        const failure = makeApiError({ status: 503, message: "email is not available on this site" });
        useForgotPassword.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<SetPasswordOffer email={EMAIL} />);

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
        renderWithProviders(<SetPasswordOffer email={EMAIL} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("The link could not be sent. Try again.");
    });

    it("holds the button while the link is being sent", () => {
        // given
        useForgotPassword.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<SetPasswordOffer email={EMAIL} />);

        // then
        expect(screen.getByRole("button", { name: "Set a password" })).toBeDisabled();
    });
});

describe("SetPasswordOffer with Turnstile", () => {
    it("draws the verification check while the site has it switched on", () => {
        // given
        const turnstile = makeTurnstileControl({ enabled: true });
        useTurnstile.mockReturnValue(turnstile);

        // when
        renderWithProviders(<SetPasswordOffer email={EMAIL} />);

        // then
        expect(turnstile.attach).toHaveBeenCalledWith(expect.any(HTMLDivElement));
    });

    it("asks for the verification check before sending anything", async () => {
        // given
        const request = makeMutationState();
        useForgotPassword.mockReturnValue(request);
        useTurnstile.mockReturnValue(makeTurnstileControl({ enabled: true, token: "" }));
        const user = userEvent.setup();
        renderWithProviders(<SetPasswordOffer email={EMAIL} />);

        // when
        await user.click(screen.getByRole("button", { name: "Set a password" }));

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("Complete the verification check.");
        expect(alert).toHaveFocus();
        expect(request.mutate).not.toHaveBeenCalled();
        expect(request.reset).toHaveBeenCalledOnce();
    });

    it("sends the verification token with the request, then resets the check for the next try", async () => {
        // given
        const request = settlingMutation();
        useForgotPassword.mockReturnValue(request);
        const turnstile = makeTurnstileControl({ enabled: true, token: "turnstile-1" });
        useTurnstile.mockReturnValue(turnstile);
        const user = userEvent.setup();
        renderWithProviders(<SetPasswordOffer email={EMAIL} />);

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
