import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { UpgradePrompt } from "./UpgradePrompt";

describe("UpgradePrompt", () => {
    it("says what the plan allows and that paid plans are coming", () => {
        // when
        renderWithProviders(<UpgradePrompt upgrade={{ plan: "free", limit: "teams", allowed: 1 }} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("The free plan allows 1 team.");
        expect(screen.getByRole("alert")).toHaveTextContent("Paid plans are coming. Until then, this limit stays.");
    });

    it("names the members limit when the organisation is full", () => {
        // when
        renderWithProviders(<UpgradePrompt upgrade={{ plan: "free", limit: "members", allowed: 10 }} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("The free plan allows 10 members.");
    });

    it("offers no way to pay while billing is off", () => {
        // when
        renderWithProviders(<UpgradePrompt upgrade={{ plan: "free", limit: "teams", allowed: 1 }} />);

        // then
        expect(screen.queryByRole("button")).not.toBeInTheDocument();
        expect(screen.queryByRole("link")).not.toBeInTheDocument();
    });

    it("takes the focus when asked, so a keyboard user hears why the action failed", () => {
        // when
        renderWithProviders(<UpgradePrompt upgrade={{ plan: "free", limit: "teams", allowed: 1 }} takeFocus />);

        // then
        expect(screen.getByRole("alert")).toHaveFocus();
    });

    it("leaves the focus where it is by default", () => {
        // when
        renderWithProviders(<UpgradePrompt upgrade={{ plan: "free", limit: "teams", allowed: 1 }} />);

        // then
        expect(screen.getByRole("alert")).not.toHaveFocus();
        expect(screen.getByRole("alert")).not.toHaveAttribute("tabindex");
    });
});
