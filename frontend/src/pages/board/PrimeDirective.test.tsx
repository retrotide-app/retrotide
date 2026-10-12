import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PrimeDirective } from "./PrimeDirective";

describe("PrimeDirective", () => {
    it("shows the Prime Directive as a note", () => {
        // when
        render(<PrimeDirective />);

        // then
        expect(screen.getByRole("note", { name: "The Prime Directive" })).toHaveTextContent(
            "everyone did the best job they could",
        );
    });
});
