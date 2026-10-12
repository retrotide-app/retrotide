import { describe, expect, it } from "vitest";
import { renderMarkdown } from "./markdown";

describe("renderMarkdown", () => {
    it("turns headings, lists and tables into HTML", () => {
        // given
        const source = "# Title\n\n- one\n- two\n\n| A | B |\n| - | - |\n| 1 | 2 |\n";

        // when
        const html = renderMarkdown(source);

        // then
        expect(html).toContain("<h1>Title</h1>");
        expect(html).toContain("<li>one</li>");
        expect(html).toContain("<table>");
        expect(html).toContain("<td>2</td>");
    });

    it("strips a script tag from the Markdown", () => {
        // given
        const source = "Hello\n\n<script>alert('owned')</script>\n";

        // when
        const html = renderMarkdown(source);

        // then
        expect(html).toContain("Hello");
        expect(html).not.toContain("<script");
        expect(html).not.toContain("alert");
    });

    it("strips an onerror attribute from an image in the Markdown", () => {
        // given
        const source = '<img src="x" onerror="alert(1)">\n';

        // when
        const html = renderMarkdown(source);

        // then
        expect(html).toContain("<img");
        expect(html).not.toContain("onerror");
    });

    it("strips a javascript link from the Markdown", () => {
        // given
        const source = "[click](javascript:alert(1))\n";

        // when
        const html = renderMarkdown(source);

        // then
        expect(html).toContain("click");
        expect(html).not.toContain("javascript:");
    });
});
