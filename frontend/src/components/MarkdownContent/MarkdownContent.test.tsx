import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { MarkdownContent } from "./MarkdownContent";

describe("MarkdownContent", () => {
    it("renders the Markdown as headings, paragraphs and links", () => {
        // given
        const source = "# Heading\n\nSome text with [a link](/legal/terms).\n";

        // when
        renderWithProviders(<MarkdownContent source={source} />);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Heading" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "a link" })).toHaveAttribute("href", "/legal/terms");
    });

    it("never puts a script or an event handler into the page", () => {
        // given
        const source = '<script>alert(1)</script>\n\n<img alt="pic" src="x" onerror="alert(2)">\n';

        // when
        const { container } = renderWithProviders(<MarkdownContent source={source} />);

        // then
        expect(container.querySelector("script")).toBeNull();
        expect(screen.getByRole("img", { name: "pic" })).not.toHaveAttribute("onerror");
    });
});
