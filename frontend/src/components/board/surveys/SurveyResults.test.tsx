import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeSurveyQuestion } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { SurveyResults } from "./SurveyResults";

function bar(row: HTMLElement): HTMLElement | null {
    return row.querySelector("[aria-hidden='true'] > span");
}

describe("SurveyResults", () => {
    it("draws a bar per scale value, sized by its share, with the average", () => {
        // given
        const question = makeSurveyQuestion({
            prompt: "How sustainable was the pace?",
            results: { counts: [0, 1, 0, 3, 0], responses: 4, average: 3.5 },
        });

        // when
        renderWithProviders(<SurveyResults question={question} />);

        // then
        const results = within(screen.getByRole("region", { name: "How sustainable was the pace?" }));
        const rows = results.getAllByRole("listitem");
        expect(rows.map(row => row.getAttribute("aria-label"))).toEqual(["1: 0", "2: 1", "3: 0", "4: 3", "5: 0"]);
        expect(bar(rows[1])).toHaveStyle({ width: "25%" });
        expect(bar(rows[3])).toHaveStyle({ width: "75%" });
        expect(results.getByText("Average: 3.5")).toBeInTheDocument();
    });

    it("draws a bar per option of a choice question, with no average", () => {
        // given
        const question = makeSurveyQuestion({
            kind: "multiple_choice",
            prompt: "What should we focus on?",
            options: ["Quality", "Speed"],
            scale_min: null,
            scale_max: null,
            results: { counts: [2, 1], responses: 2, average: null },
        });

        // when
        renderWithProviders(<SurveyResults question={question} />);

        // then
        const results = within(screen.getByRole("region", { name: "What should we focus on?" }));
        expect(results.getByRole("listitem", { name: "Quality: 2" })).toBeInTheDocument();
        expect(bar(results.getByRole("listitem", { name: "Speed: 1" }))).toHaveStyle({ width: "50%" });
        expect(results.queryByText(/Average/)).not.toBeInTheDocument();
    });

    it("says no scores came in when nobody rated", () => {
        // given
        const question = makeSurveyQuestion({ results: { counts: [0, 0, 0, 0, 0], responses: 0, average: null } });

        // when
        renderWithProviders(<SurveyResults question={question} />);

        // then
        expect(screen.getByText("Average: No scores yet")).toBeInTheDocument();
    });
});
