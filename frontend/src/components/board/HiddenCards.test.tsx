import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { HiddenCards } from "./HiddenCards";

describe("HiddenCards", () => {
    it("draws one empty placeholder per hidden card and says how many there are", () => {
        // when
        const { container } = render(
            <HiddenCards placeholders={[{ key: "c2:hidden:0" }, { key: "c2:hidden:1" }, { key: "c2:hidden:2" }]} />,
        );

        // then
        expect(screen.getByText("3 cards from others, hidden for now")).toBeInTheDocument();
        const placeholders = container.querySelectorAll("li");
        expect(placeholders).toHaveLength(3);
        for (const placeholder of placeholders) {
            expect(placeholder).toBeEmptyDOMElement();
        }
        expect(screen.queryByRole("listitem")).not.toBeInTheDocument();
    });

    it("speaks of a single hidden card in the singular", () => {
        // when
        render(<HiddenCards placeholders={[{ key: "c2:hidden:0" }]} />);

        // then
        expect(screen.getByText("1 card from someone else, hidden for now")).toBeInTheDocument();
    });

    it("draws nothing when no card is hidden", () => {
        // when
        const { container } = render(<HiddenCards placeholders={[]} />);

        // then
        expect(container).toBeEmptyDOMElement();
    });
});
