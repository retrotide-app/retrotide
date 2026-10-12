import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Panel } from "./Panel";

describe("Panel", () => {
    it("names its region after its heading", () => {
        // given
        const title = "Password";

        // when
        renderWithProviders(
            <Panel title={title} headingLevel={3}>
                <p>the panel body</p>
            </Panel>,
        );

        // then
        expect(screen.getByRole("region", { name: title })).toHaveTextContent("the panel body");
        expect(screen.getByRole("heading", { level: 3, name: title })).toBeInTheDocument();
    });

    it("heads itself at the level it is given", () => {
        // given
        const headingLevel = 2;

        // when
        renderWithProviders(<Panel title="Email address" headingLevel={headingLevel} />);

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Email address" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { level: 3 })).not.toBeInTheDocument();
    });

    it("explains itself when given a description", () => {
        // given
        const description = "Signs the user out everywhere.";

        // when
        renderWithProviders(<Panel title="Sessions" headingLevel={3} description={description} />);

        // then
        expect(screen.getByRole("region", { name: "Sessions" })).toHaveTextContent(description);
    });

    it("moves focus to its heading when asked to", () => {
        // when
        renderWithProviders(<Panel title="Pick a template" headingLevel={2} takeFocus />);

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Pick a template" })).toHaveFocus();
    });

    it("leaves focus where it is by default", () => {
        // when
        renderWithProviders(<Panel title="Sessions" headingLevel={3} />);

        // then
        expect(screen.getByRole("heading", { level: 3, name: "Sessions" })).not.toHaveFocus();
        expect(screen.getByRole("heading", { level: 3, name: "Sessions" })).not.toHaveAttribute("tabindex");
    });

    it("heads itself with markup, such as a link, and is still named by it", () => {
        // given
        const title = <a href="/o/harbour/boards/b1">Sprint 12</a>;

        // when
        renderWithProviders(<Panel title={title} headingLevel={2} />);

        // then
        const heading = screen.getByRole("heading", { level: 2, name: "Sprint 12" });
        expect(heading).toContainElement(screen.getByRole("link", { name: "Sprint 12" }));
        expect(screen.getByRole("region", { name: "Sprint 12" })).toBeInTheDocument();
    });
});
