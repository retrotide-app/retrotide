import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TWO_FACTOR_PATH } from "../../../domain/twoFactor";
import { makeApiError, makeMutationState, makeSessionUser } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { TwoFactorPage } from "./TwoFactorPage";

const { useVerifyTwoFactor } = vi.hoisted(() => ({ useVerifyTwoFactor: vi.fn() }));

vi.mock("../../../hooks/mutations/twoFactor", () => ({ useVerifyTwoFactor }));

function verifying(location?: string) {
    return makeMutationState({
        mutate: vi.fn((_variables: unknown, options?: { onSuccess?: (result: unknown) => void }) => {
            options?.onSuccess?.({ user: makeSessionUser(), location });
        }),
    });
}

function renderPage(routeState?: unknown) {
    return renderWithProviders(
        <Routes>
            <Route path={TWO_FACTOR_PATH} element={<TwoFactorPage />} />
            <Route path="/" element={<p>the home page</p>} />
            <Route path="/account" element={<p>the account page</p>} />
            <Route path="/o/acme/boards" element={<p>the boards page</p>} />
            <Route path="/login" element={<p>the sign-in page</p>} />
        </Routes>,
        { route: TWO_FACTOR_PATH, routeState },
    );
}

async function submitCode(code: string) {
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Code"), code);
    await user.click(screen.getByRole("button", { name: "Verify" }));

    return user;
}

beforeEach(() => {
    useVerifyTwoFactor.mockReturnValue(makeMutationState());
});

describe("TwoFactorPage", () => {
    it("asks for the code from the authenticator app", () => {
        // when
        renderPage();

        // then
        const code = screen.getByLabelText("Code");
        expect(screen.getByRole("heading", { level: 1, name: "Two-factor authentication" })).toBeInTheDocument();
        expect(code).toHaveAttribute("inputmode", "numeric");
        expect(code).toHaveAttribute("autocomplete", "one-time-code");
        expect(screen.getByRole("button", { name: "Verify" })).toBeInTheDocument();
        expect(screen.getByRole("switch", { name: "Don't ask again on this device" })).toHaveAttribute(
            "aria-checked",
            "false",
        );
        expect(screen.getByText("Skip the code when you sign in here for the next 30 days.")).toBeInTheDocument();
    });

    it("signs in with a recovery code instead, on a full keyboard", async () => {
        // given someone who has lost their authenticator app
        const verify = verifying("/o/acme/boards");
        useVerifyTwoFactor.mockReturnValue(verify);
        renderPage();
        const user = userEvent.setup();

        // when
        await user.click(screen.getByRole("button", { name: "Lost your phone? Use a recovery code" }));
        const recovery = screen.getByLabelText("Recovery code");
        await user.type(recovery, "abcde-fgh23");
        await user.click(screen.getByRole("button", { name: "Verify" }));

        // then
        expect(recovery).toHaveAttribute("inputmode", "text");
        expect(verify.mutate).toHaveBeenCalledWith({ code: "abcde-fgh23", trust_device: false }, expect.anything());
        expect(await screen.findByText("the boards page")).toBeInTheDocument();
    });

    it("asks for a recovery code before checking anything, and can go back to the app code", async () => {
        // given
        const verify = makeMutationState();
        useVerifyTwoFactor.mockReturnValue(verify);
        renderPage();
        const user = userEvent.setup();
        await user.click(screen.getByRole("button", { name: "Lost your phone? Use a recovery code" }));

        // when
        await user.click(screen.getByRole("button", { name: "Verify" }));

        // then
        expect(verify.mutate).not.toHaveBeenCalled();
        expect(screen.getByLabelText("Recovery code")).toHaveAccessibleDescription(
            "Enter one of the recovery codes you saved when you turned on two-factor authentication.",
        );
        await user.click(screen.getByRole("button", { name: "Use the code from your authenticator app" }));
        expect(screen.getByLabelText("Code")).toHaveAttribute("inputmode", "numeric");
    });

    it("asks for a code before checking anything", async () => {
        // given
        const verify = makeMutationState();
        useVerifyTwoFactor.mockReturnValue(verify);
        renderPage();

        // when
        await userEvent.setup().click(screen.getByRole("button", { name: "Verify" }));

        // then
        expect(verify.mutate).not.toHaveBeenCalled();
        expect(screen.getByLabelText("Code")).toHaveAccessibleDescription(
            "Enter the 6-digit code from your authenticator app.",
        );
    });

    it("lands on the path the server hands back", async () => {
        // given
        const verify = verifying("/o/acme/boards");
        useVerifyTwoFactor.mockReturnValue(verify);
        renderPage({ from: "/account" });

        // when
        await submitCode(" 081804 ");

        // then the device is not trusted unless asked
        expect(verify.mutate).toHaveBeenCalledWith({ code: "081804", trust_device: false }, expect.anything());
        expect(await screen.findByText("the boards page")).toBeInTheDocument();
    });

    it("trusts this device for 30 days when asked not to ask again", async () => {
        // given
        const verify = verifying("/o/acme/boards");
        useVerifyTwoFactor.mockReturnValue(verify);
        renderPage();
        const user = userEvent.setup();
        const trust = screen.getByRole("switch", { name: "Don't ask again on this device" });

        // when
        await user.click(trust);
        await user.type(screen.getByLabelText("Code"), "081804");
        await user.click(screen.getByRole("button", { name: "Verify" }));

        // then
        expect(verify.mutate).toHaveBeenCalledWith({ code: "081804", trust_device: true }, expect.anything());
    });

    it("falls back to the page that sent the visitor to sign in", async () => {
        // given
        useVerifyTwoFactor.mockReturnValue(verifying());
        renderPage({ from: "/account" });

        // when
        await submitCode("081804");

        // then
        expect(await screen.findByText("the account page")).toBeInTheDocument();
    });

    it("falls back to the home page without anywhere to return to", async () => {
        // given
        useVerifyTwoFactor.mockReturnValue(verifying());
        renderPage();

        // when
        await submitCode("081804");

        // then
        expect(await screen.findByText("the home page")).toBeInTheDocument();
    });

    it("ignores a landing path that leaves the site", async () => {
        // given
        useVerifyTwoFactor.mockReturnValue(verifying("//evil.example/steal"));
        renderPage({ from: "/account" });

        // when
        await submitCode("081804");

        // then
        expect(await screen.findByText("the account page")).toBeInTheDocument();
    });

    it("ties a wrong code to the code field", () => {
        // given
        const failure = makeApiError({ status: 400, message: "that code is not right", code: "invalid_code" });
        useVerifyTwoFactor.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("that code is not right");
        expect(screen.getByLabelText("Code")).toHaveAccessibleDescription("that code is not right");
    });

    it("sends the visitor back to sign in once the sign-in has expired", () => {
        // given
        const failure = makeApiError({
            status: 401,
            message: "signing in took too long, sign in again",
            code: "two_factor_expired",
        });
        useVerifyTwoFactor.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Signing in took too long");
        expect(screen.getByRole("link", { name: "Sign in again" })).toHaveAttribute("href", "/login");
        expect(screen.queryByLabelText("Code")).not.toBeInTheDocument();
    });

    it("shows any other refusal as it came", () => {
        // given
        const failure = makeApiError({ status: 403, message: "your account has been banned" });
        useVerifyTwoFactor.mockReturnValue(makeMutationState({ isError: true, error: failure }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("your account has been banned");
        expect(screen.getByLabelText("Code")).not.toHaveAttribute("aria-invalid");
    });
});
