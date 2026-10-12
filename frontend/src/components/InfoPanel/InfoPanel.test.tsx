import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { InfoPanel } from "./InfoPanel";

describe("InfoPanel", () => {
    it("presents its title as a heading", () => {
        // given
        const title = "How this works";

        // when
        renderWithProviders(
            <InfoPanel title={title}>
                <p>Add a card to any column</p>
            </InfoPanel>,
        );

        // then
        expect(screen.getByRole("heading", { name: title })).toBeInTheDocument();
    });

    it("renders its children below the heading", () => {
        // given
        const body = "Add a card to any column";

        // when
        renderWithProviders(
            <InfoPanel title="How this works">
                <p>{body}</p>
            </InfoPanel>,
        );

        // then
        const heading = screen.getByRole("heading", { name: "How this works" });
        const paragraph = screen.getByText(body);
        expect(paragraph).toBeInTheDocument();
        expect(heading.compareDocumentPosition(paragraph) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });

    it("renders several children in the order they were given", () => {
        // given
        const items = ["first step", "second step"];

        // when
        renderWithProviders(
            <InfoPanel title="Steps">
                <p>{items[0]}</p>
                <p>{items[1]}</p>
            </InfoPanel>,
        );

        // then
        const paragraphs = screen.getAllByText(/step$/);
        expect(paragraphs.map(node => node.textContent)).toEqual(items);
    });

    it("renders interactive children that keep working", () => {
        // given
        const markup = (
            <InfoPanel title="Steps">
                <a href="/help">Read the guide</a>
            </InfoPanel>
        );

        // when
        renderWithProviders(markup);

        // then
        expect(screen.getByRole("link", { name: "Read the guide" })).toHaveAttribute("href", "/help");
    });
});
