import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { RevealBoard } from "./RevealBoard";

describe("RevealBoard", () => {
    it("shows the viewer's own card and hides everyone else's", () => {
        // given
        const ui = <RevealBoard />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText("Deploys were smooth all sprint")).toBeInTheDocument();
        expect(screen.getByText("Only you can see this")).toBeInTheDocument();
        expect(screen.getAllByText("Hidden card")).toHaveLength(5);
    });

    it("keeps the text of a hidden card out of the page entirely", () => {
        // given
        const ui = <RevealBoard />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.queryByText("Stand-ups keep running over")).not.toBeInTheDocument();
        expect(screen.queryByText("Timebox stand-ups to 15 minutes")).not.toBeInTheDocument();
    });

    it("groups the cards under their named columns", () => {
        // given
        const ui = <RevealBoard />;

        // when
        renderWithProviders(ui);

        // then
        expect(within(screen.getByRole("group", { name: "Went well" })).getAllByRole("listitem")).toHaveLength(2);
        expect(within(screen.getByRole("group", { name: "To improve" })).getAllByRole("listitem")).toHaveLength(2);
        expect(within(screen.getByRole("group", { name: "Actions" })).getAllByRole("listitem")).toHaveLength(2);
    });

    it("reveals every card with its votes when the reveal button is pressed", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<RevealBoard />);

        // when
        await user.click(screen.getByRole("button", { name: "Reveal cards" }));

        // then
        expect(screen.queryByText("Hidden card")).not.toBeInTheDocument();
        expect(screen.getByText("Stand-ups keep running over")).toBeInTheDocument();
        expect(screen.getByText("4 votes")).toBeInTheDocument();
        expect(screen.getByText("1 vote")).toBeInTheDocument();
        expect(screen.queryByText("Only you can see this")).not.toBeInTheDocument();
        expect(screen.getByText("All cards revealed")).toBeInTheDocument();
    });

    it("marks the button as pressed while the cards are revealed", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<RevealBoard />);

        // when
        await user.click(screen.getByRole("button", { name: "Reveal cards" }));

        // then
        expect(screen.getByRole("button", { name: "Hide cards" })).toHaveAttribute("aria-pressed", "true");
    });

    it("hides the cards again when the button is pressed a second time", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<RevealBoard />);
        await user.click(screen.getByRole("button", { name: "Reveal cards" }));

        // when
        await user.click(screen.getByRole("button", { name: "Hide cards" }));

        // then
        expect(screen.getAllByText("Hidden card")).toHaveLength(5);
        expect(screen.getByRole("button", { name: "Reveal cards" })).toHaveAttribute("aria-pressed", "false");
        expect(screen.getByText("5 people writing, cards hidden")).toBeInTheDocument();
    });
});
