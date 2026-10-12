import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { OrgPage } from "./OrgPage";

beforeEach(() => {
    document.title = "";
});

describe("OrgPage", () => {
    it("heads the page and names the browser tab after it", () => {
        // when
        renderWithProviders(<OrgPage title="Members" />);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Members" })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Members" })).toBeInTheDocument();
        expect(document.title).toContain("Members");
    });

    it("draws the status beside the heading and the content below it", () => {
        // when
        renderWithProviders(
            <OrgPage title="Crew" status={<span>Archived</span>}>
                <p>the team details</p>
            </OrgPage>,
        );

        // then
        const page = screen.getByRole("region", { name: "Crew" });
        expect(page).toHaveTextContent("Archived");
        expect(page).toHaveTextContent("the team details");
    });
});
