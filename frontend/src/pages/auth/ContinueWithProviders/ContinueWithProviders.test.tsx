import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { makeApiError, makeIdentityProvider, makeProviderSignIn } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { ContinueWithProviders } from "./ContinueWithProviders";

const google = makeIdentityProvider();
const example = makeIdentityProvider({ id: "example", name: "Example ID" });

describe("ContinueWithProviders", () => {
    it("starts the provider's sign-in when pressed", async () => {
        // given
        const onStart = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <ContinueWithProviders providers={[google]} control={makeProviderSignIn()} onStart={onStart} />,
        );

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Google" }));

        // then
        expect(onStart).toHaveBeenCalledExactlyOnceWith(google);
    });

    it("offers one button for each provider, in the order the site lists them", () => {
        // given
        const providers = [example, google];

        // when
        renderWithProviders(
            <ContinueWithProviders providers={providers} control={makeProviderSignIn()} onStart={vi.fn()} />,
        );

        // then
        const names = screen.getAllByRole("button").map(button => button.textContent);
        expect(names).toEqual(["Continue with Example ID", "Continue with Google"]);
    });

    it("starts the sign-in of the provider that was pressed when there are two", async () => {
        // given
        const onStart = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <ContinueWithProviders providers={[google, example]} control={makeProviderSignIn()} onStart={onStart} />,
        );

        // when
        await user.click(screen.getByRole("button", { name: "Continue with Example ID" }));

        // then
        expect(onStart).toHaveBeenCalledExactlyOnceWith(example);
    });

    it("holds the button while the page is on its way to the provider", () => {
        // given
        const control = makeProviderSignIn({ pending: true, provider: google });

        // when
        renderWithProviders(<ContinueWithProviders providers={[google]} control={control} onStart={vi.fn()} />);

        // then
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeDisabled();
    });

    it("holds every provider's button while the page is on its way to one of them", () => {
        // given
        const control = makeProviderSignIn({ pending: true, provider: example });

        // when
        renderWithProviders(
            <ContinueWithProviders providers={[google, example]} control={control} onStart={vi.fn()} />,
        );

        // then
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Continue with Example ID" })).toBeDisabled();
    });

    it("separates itself from the email form", () => {
        // given
        const control = makeProviderSignIn();

        // when
        renderWithProviders(<ContinueWithProviders providers={[google]} control={control} onStart={vi.fn()} />);

        // then
        expect(screen.getByText("or")).toBeInTheDocument();
    });

    it("leaves the divider out when there is no email form to separate it from", () => {
        // given
        const control = makeProviderSignIn();

        // when
        renderWithProviders(
            <ContinueWithProviders providers={[google]} control={control} onStart={vi.fn()} showDivider={false} />,
        );

        // then
        expect(screen.queryByText("or")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Continue with Google" })).toBeInTheDocument();
    });

    it("shows the server's reason when the sign-in could not start, and moves focus to it", () => {
        // given
        const failure = makeApiError({ status: 429, message: "too many requests, please try again later" });
        const control = makeProviderSignIn({ error: failure, provider: google });

        // when
        renderWithProviders(<ContinueWithProviders providers={[google]} control={control} onStart={vi.fn()} />);

        // then
        const alert = screen.getByRole("alert");
        expect(alert).toHaveTextContent("too many requests, please try again later");
        expect(alert).toHaveFocus();
    });

    it("says the provider's sign-in is off when the site switched it off after the page loaded", () => {
        // given
        const failure = makeApiError({ status: 404, message: "not found" });
        const control = makeProviderSignIn({ error: failure, provider: google });

        // when
        renderWithProviders(<ContinueWithProviders providers={[google]} control={control} onStart={vi.fn()} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Signing in with Google is switched off on this site.");
    });

    it("names the provider whose sign-in could not start when there are two", () => {
        // given
        const failure = makeApiError({ status: 404, message: "not found" });
        const control = makeProviderSignIn({ error: failure, provider: example });

        // when
        renderWithProviders(
            <ContinueWithProviders providers={[google, example]} control={control} onStart={vi.fn()} />,
        );

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("Signing in with Example ID is switched off on this site.");
    });

    it("shows no alert while nothing has gone wrong", () => {
        // given
        const control = makeProviderSignIn();

        // when
        renderWithProviders(<ContinueWithProviders providers={[google]} control={control} onStart={vi.fn()} />);

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
});
