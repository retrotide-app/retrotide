import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeOrganisation } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { FirstBoardGuide } from "./FirstBoardGuide";

const harbour = makeOrganisation({ slug: "harbour", name: "Harbour" });

describe("FirstBoardGuide", () => {
    it("offers one large tile that opens the wizard for a board with no team, with one short line", () => {
        // when
        renderWithProviders(<FirstBoardGuide organisation={harbour} canCreate />);

        // then
        const guide = screen.getByRole("region", { name: "No boards yet" });
        expect(guide).toHaveTextContent("Pick a template, give it a name, then share the link with your team.");
        expect(screen.getByRole("link", { name: "Create your first board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=none",
        );
        expect(screen.getAllByRole("link")).toHaveLength(1);
    });

    it("offers nothing to do to a viewer who may not create boards", () => {
        // when
        renderWithProviders(<FirstBoardGuide organisation={harbour} canCreate={false} />);

        // then
        expect(screen.getByRole("region", { name: "No boards yet" })).toHaveTextContent(
            "Boards shared with you will show up here.",
        );
        expect(screen.queryByRole("link")).not.toBeInTheDocument();
    });
});
