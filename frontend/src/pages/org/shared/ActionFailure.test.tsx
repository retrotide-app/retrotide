import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeApiError, makeUpgradeRequiredError } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { ActionFailure } from "./ActionFailure";

describe("ActionFailure", () => {
    it("says paid plans are coming when the plan's limit refused the action", () => {
        // when
        renderWithProviders(<ActionFailure error={makeUpgradeRequiredError({ limit: "teams", allowed: 1 })} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("The free plan allows 1 team.");
        expect(screen.getByRole("alert")).toHaveTextContent("Paid plans are coming.");
    });

    it("shows the server's reason for any other refusal", () => {
        // when
        renderWithProviders(
            <ActionFailure error={makeApiError({ status: 403, message: "only the owner can make an admin" })} />,
        );

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("only the owner can make an admin");
        expect(screen.getByRole("alert")).not.toHaveTextContent("Paid plans are coming.");
    });

    it("says plainly that nothing was sent when the site cannot send email", () => {
        // given
        const error = makeApiError({
            status: 503,
            message: "email is not available on this site",
            code: "email_unavailable",
        });

        // when
        renderWithProviders(<ActionFailure error={error} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("This site cannot send email yet, so nothing was sent.");
    });

    it("shows the server's reason for a 402 that is not about the plan's limits", () => {
        // when
        renderWithProviders(<ActionFailure error={makeApiError({ status: 402, message: "payment needed" })} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("payment needed");
    });

    it("moves focus to the reason when asked to", () => {
        // when
        renderWithProviders(<ActionFailure error={makeApiError({ status: 500, message: "failed" })} takeFocus />);

        // then
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("moves focus to the upgrade note when asked to", () => {
        // when
        renderWithProviders(<ActionFailure error={makeUpgradeRequiredError()} takeFocus />);

        // then
        expect(screen.getByRole("alert")).toHaveFocus();
    });
});
