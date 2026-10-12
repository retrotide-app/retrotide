import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { PageState } from "./PageState";

describe("PageState", () => {
    it("names its region after the page heading", () => {
        // when
        renderWithProviders(<PageState title="Organisation not found" />);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Organisation not found" })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Organisation not found" })).toBeInTheDocument();
    });

    it("draws a second level heading inside a page that already has one", () => {
        // when
        renderWithProviders(<PageState title="Organisation not found" headingLevel={2} />);

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Organisation not found" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
    });

    it("shows the description and the actions it is given", () => {
        // when
        renderWithProviders(
            <PageState title="Organisation not found" description="The address may be wrong.">
                <button type="button">Go back</button>
            </PageState>,
        );

        // then
        expect(screen.getByText("The address may be wrong.")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Go back" })).toBeInTheDocument();
    });
});
