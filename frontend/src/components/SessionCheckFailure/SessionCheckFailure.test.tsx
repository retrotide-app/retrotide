import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { SessionCheckFailure } from "./SessionCheckFailure";

describe("SessionCheckFailure", () => {
    it("says the sign-in could not be checked, with the server's reason", () => {
        // given
        const message = "service unavailable";

        // when
        renderWithProviders(<SessionCheckFailure message={message} onRetry={vi.fn()} />);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "We could not check your sign-in" })).toBeInTheDocument();
        expect(screen.getByRole("alert")).toHaveTextContent(message);
    });

    it("heads itself one level down inside a page that already has a heading", () => {
        // given
        const headingLevel = 2;

        // when
        renderWithProviders(
            <SessionCheckFailure message="service unavailable" onRetry={vi.fn()} headingLevel={headingLevel} />,
        );

        // then
        expect(screen.getByRole("heading", { level: 2, name: "We could not check your sign-in" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
    });

    it("checks again when asked", async () => {
        // given
        const onRetry = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<SessionCheckFailure message="service unavailable" onRetry={onRetry} />);

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(onRetry).toHaveBeenCalledOnce();
    });
});
