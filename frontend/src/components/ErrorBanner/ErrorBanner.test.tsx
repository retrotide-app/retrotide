import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { ErrorBanner } from "./ErrorBanner";

describe("ErrorBanner", () => {
    it("shows the message it is given", () => {
        // given
        const message = "The request could not be completed";

        // when
        renderWithProviders(<ErrorBanner message={message} />);

        // then
        expect(screen.getByText(message)).toBeInTheDocument();
    });

    it("announces its message as an alert", () => {
        // given
        const message = "The request could not be completed";

        // when
        renderWithProviders(<ErrorBanner message={message} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(message);
    });

    it("replaces the message when a new failure arrives", () => {
        // given
        const { rerender } = renderWithProviders(<ErrorBanner message="The request could not be completed" />);

        // when
        rerender(<ErrorBanner message="The server refused the change" />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("The server refused the change");
        expect(screen.queryByText("The request could not be completed")).not.toBeInTheDocument();
    });

    it("still renders its container when the message is empty", () => {
        // given
        const message = "";

        // when
        renderWithProviders(<ErrorBanner message={message} />);

        // then
        expect(screen.getByRole("alert")).toBeEmptyDOMElement();
    });

    it("leaves focus where it was by default", () => {
        // given
        const ui = <ErrorBanner message="The request could not be completed" />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("alert")).not.toHaveFocus();
        expect(screen.getByRole("alert")).not.toHaveAttribute("tabindex");
    });

    it("takes focus when asked, so a keyboard or screen reader user lands on the failure", () => {
        // given
        const ui = <ErrorBanner message="Wrong email or password" takeFocus />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("alert")).toHaveFocus();
        expect(screen.getByRole("alert")).toHaveAttribute("tabindex", "-1");
    });

    it("takes focus again when a new failure replaces the old one", () => {
        // given
        const { rerender } = renderWithProviders(<ErrorBanner message="Wrong email or password" takeFocus />);
        screen.getByRole("alert").blur();

        // when
        rerender(<ErrorBanner message="Too many attempts" takeFocus />);

        // then
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});
