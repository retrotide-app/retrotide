import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { makeApiError, makeMutationState, makeTurnstileControl } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { ResendVerification } from "./ResendVerification";

const { useResendVerification, useTurnstile } = vi.hoisted(() => ({
    useResendVerification: vi.fn(),
    useTurnstile: vi.fn(),
}));

vi.mock("../../../hooks/mutations/auth", () => ({ useResendVerification }));
vi.mock("../../../hooks/useTurnstile", () => ({ useTurnstile }));

function settlingMutation() {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSettled?: () => void }) => {
            options?.onSettled?.();
        }),
    });
}

beforeEach(() => {
    useResendVerification.mockReturnValue(makeMutationState());
    useTurnstile.mockReturnValue(makeTurnstileControl());
});

describe("ResendVerification for a known address", () => {
    it("offers only a button, because it already knows the address", () => {
        // given
        const email = "sam@example.test";

        // when
        renderWithProviders(<ResendVerification email={email} />);

        // then
        expect(screen.getByRole("button", { name: "Resend link" })).toBeInTheDocument();
        expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    });

    it("sends a new link to the address, with no session needed", async () => {
        // given
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        const user = userEvent.setup();
        renderWithProviders(<ResendVerification email=" sam@example.test " />, { user: null });

        // when
        await user.click(screen.getByRole("button", { name: "Resend link" }));

        // then
        expect(resend.mutate).toHaveBeenCalledExactlyOnceWith(
            { email: "sam@example.test", turnstile_token: undefined },
            expect.anything(),
        );
    });

    it("says the link was sent to the address and moves focus to the notice", () => {
        // given
        useResendVerification.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderWithProviders(<ResendVerification email="sam@example.test" />);

        // then
        const notice = screen.getByRole("status");
        expect(notice).toHaveTextContent("Link sent to sam@example.test. Check your inbox.");
        expect(notice).toHaveFocus();
        expect(screen.queryByRole("button", { name: "Resend link" })).not.toBeInTheDocument();
    });

    it("holds the button while the link is being sent", () => {
        // given
        useResendVerification.mockReturnValue(makeMutationState({ isPending: true }));

        // when
        renderWithProviders(<ResendVerification email="sam@example.test" />);

        // then
        expect(screen.getByRole("button", { name: "Resend link" })).toBeDisabled();
    });

    it("says plainly that nothing was sent when the site cannot send email, and lets the visitor try again", () => {
        // given
        const failure = makeApiError({
            status: 503,
            message: "email is not available on this site",
            code: "email_unavailable",
        });
        useResendVerification.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<ResendVerification email="sam@example.test" />);

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("This site cannot send email yet, so nothing was sent.");
        expect(alert).toHaveFocus();
        expect(screen.getByRole("button", { name: "Resend link" })).toBeEnabled();
    });

    it("shows the server's reason when too many links were asked for", () => {
        // given
        const failure = makeApiError({ status: 429, message: "too many requests, please try again later" });
        useResendVerification.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<ResendVerification email="sam@example.test" />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("too many requests, please try again later");
    });
});

describe("ResendVerification asking for the address", () => {
    it("asks for the email address the account uses", () => {
        // given
        const ui = <ResendVerification />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText("Enter your email address and we will send you a new link.")).toBeInTheDocument();
        expect(screen.getByLabelText("Email address")).toHaveAttribute("type", "email");
        expect(screen.getByRole("button", { name: "Send new link" })).toBeInTheDocument();
    });

    it("sends a new link to the address entered, without the spaces around it", async () => {
        // given
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        const user = userEvent.setup();
        renderWithProviders(<ResendVerification />, { user: null });

        // when
        await user.type(screen.getByLabelText("Email address"), " sam@example.test ");
        await user.click(screen.getByRole("button", { name: "Send new link" }));

        // then
        expect(resend.mutate).toHaveBeenCalledExactlyOnceWith(
            { email: "sam@example.test", turnstile_token: undefined },
            expect.anything(),
        );
    });

    it("asks for an address before sending anything, and moves focus to the summary", async () => {
        // given
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        const user = userEvent.setup();
        renderWithProviders(<ResendVerification />);

        // when
        await user.click(screen.getByRole("button", { name: "Send new link" }));

        // then
        expect(screen.getByLabelText("Email address")).toHaveAccessibleDescription("Enter your email address.");
        expect(screen.getByRole("alert")).toHaveTextContent("Enter your email address.");
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(resend.mutate).not.toHaveBeenCalled();
        expect(resend.reset).toHaveBeenCalled();
    });

    it("gives the same answer whether or not the address has an account that needs verifying", async () => {
        // given
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        const user = userEvent.setup();
        const { rerender } = renderWithProviders(<ResendVerification />);
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.click(screen.getByRole("button", { name: "Send new link" }));

        // when
        useResendVerification.mockReturnValue(makeMutationState({ isSuccess: true }));
        rerender(<ResendVerification />);

        // then
        const notice = screen.getByRole("status");
        expect(notice).toHaveTextContent(
            "Check your inbox. If an account uses sam@example.test and needs verifying, we sent it a new link.",
        );
        expect(notice).toHaveFocus();
        expect(screen.queryByLabelText("Email address")).not.toBeInTheDocument();
    });

    it("shows the server's reason for an address it cannot use", () => {
        // given
        const failure = makeApiError({ status: 400, message: "a valid email address is required" });
        useResendVerification.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderWithProviders(<ResendVerification />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("a valid email address is required");
    });
});

describe("ResendVerification with Turnstile", () => {
    it("draws no verification check while the site has it switched off", () => {
        // given
        const turnstile = makeTurnstileControl({ enabled: false });
        useTurnstile.mockReturnValue(turnstile);

        // when
        renderWithProviders(<ResendVerification email="sam@example.test" />);

        // then
        expect(turnstile.attach).not.toHaveBeenCalled();
    });

    it("draws the verification check while the site has it switched on", () => {
        // given
        const turnstile = makeTurnstileControl({ enabled: true });
        useTurnstile.mockReturnValue(turnstile);

        // when
        renderWithProviders(<ResendVerification email="sam@example.test" />);

        // then
        expect(turnstile.attach).toHaveBeenCalledWith(expect.any(HTMLDivElement));
    });

    it("asks for the verification check before sending anything", async () => {
        // given
        const resend = makeMutationState();
        useResendVerification.mockReturnValue(resend);
        useTurnstile.mockReturnValue(makeTurnstileControl({ enabled: true, token: "" }));
        const user = userEvent.setup();
        renderWithProviders(<ResendVerification email="sam@example.test" />);

        // when
        await user.click(screen.getByRole("button", { name: "Resend link" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Complete the verification check.");
        expect(resend.mutate).not.toHaveBeenCalled();
    });

    it("sends the verification token with the request, then resets the check for the next try", async () => {
        // given
        const resend = settlingMutation();
        useResendVerification.mockReturnValue(resend);
        const turnstile = makeTurnstileControl({ enabled: true, token: "turnstile-1" });
        useTurnstile.mockReturnValue(turnstile);
        const user = userEvent.setup();
        renderWithProviders(<ResendVerification />);

        // when
        await user.type(screen.getByLabelText("Email address"), "sam@example.test");
        await user.click(screen.getByRole("button", { name: "Send new link" }));

        // then
        expect(resend.mutate).toHaveBeenCalledWith(
            { email: "sam@example.test", turnstile_token: "turnstile-1" },
            expect.anything(),
        );
        expect(turnstile.reset).toHaveBeenCalledOnce();
    });
});
