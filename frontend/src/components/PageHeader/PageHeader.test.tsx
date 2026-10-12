import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { PageHeader } from "./PageHeader";

describe("PageHeader", () => {
    it("shows the title as a top level heading with its description under it", () => {
        // when
        renderWithProviders(
            <PageHeader titleId="teams-title" title="Teams" description="Groups of people who share boards." />,
        );

        // then
        const heading = screen.getByRole("heading", { level: 1, name: "Teams" });
        expect(heading).toHaveAttribute("id", "teams-title");
        expect(screen.getByText("Groups of people who share boards.")).toBeInTheDocument();
    });

    it("uses a second level heading when asked", () => {
        // when
        renderWithProviders(<PageHeader titleId="users-title" title="Users" headingLevel={2} />);

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Users" })).toBeInTheDocument();
    });

    it("leaves the description out when there is none", () => {
        // when
        const { container } = renderWithProviders(<PageHeader titleId="t" title="Users" />);

        // then
        expect(container.querySelector("p")).toBeNull();
    });

    it("shows the actions it is given beside the title", () => {
        // when
        renderWithProviders(<PageHeader titleId="t" title="Teams" actions={<button type="button">New team</button>} />);

        // then
        expect(screen.getByRole("button", { name: "New team" })).toBeInTheDocument();
    });
});
