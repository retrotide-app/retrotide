import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { LoadFailure } from "./LoadFailure";

describe("LoadFailure", () => {
    it("shows the reason the load failed", () => {
        // given
        const message = "failed to list the users";

        // when
        renderWithProviders(<LoadFailure message={message} onRetry={vi.fn()} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(message);
    });

    it("tries again when asked", async () => {
        // given
        const onRetry = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<LoadFailure message="failed to list the users" onRetry={onRetry} />);

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(onRetry).toHaveBeenCalledOnce();
    });
});
