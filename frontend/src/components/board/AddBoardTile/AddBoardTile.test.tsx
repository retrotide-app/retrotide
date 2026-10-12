import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { AddBoardTile } from "./AddBoardTile";

describe("AddBoardTile", () => {
    it("is a link named by its label, without the plus sign", () => {
        // when
        renderWithProviders(<AddBoardTile href="/o/harbour/boards/new?team=team-crew" label="Add board" />);

        // then
        expect(screen.getByRole("link", { name: "Add board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=team-crew",
        );
    });

    it("opens the page it points at", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(
            <Routes>
                <Route path="/" element={<AddBoardTile href="/boards/new" label="Add board" />} />
                <Route path="/boards/new" element={<h1>New board wizard</h1>} />
            </Routes>,
        );

        // when
        await user.click(screen.getByRole("link", { name: "Add board" }));

        // then
        expect(screen.getByRole("heading", { name: "New board wizard" })).toBeInTheDocument();
    });
});
