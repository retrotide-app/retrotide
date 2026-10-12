import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeBoardView } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { StageBar } from "./StageBar";

function currentStep(): string | null {
    const steps = within(screen.getByRole("list", { name: "Retro stages" })).getAllByRole("listitem");

    return steps.find(step => step.getAttribute("aria-current") === "step")?.textContent ?? null;
}

describe("StageBar", () => {
    it("lists the five stages in order", () => {
        // when
        renderWithProviders(<StageBar board={makeBoardView()} />);

        // then
        const steps = within(screen.getByRole("list", { name: "Retro stages" })).getAllByRole("listitem");
        expect(steps.map(step => step.textContent)).toEqual([
            "1Step 1 of 5: Write",
            "2Step 2 of 5: Vote",
            "3Step 3 of 5: Discuss",
            "4Step 4 of 5: Actions",
            "5Step 5 of 5: End",
        ]);
    });

    it.each([
        ["write", "Write", "Write your cards. Nobody can vote or comment yet."],
        ["vote", "Vote", "The cards are revealed. Vote for the ones you most want to talk about."],
        [
            "discuss",
            "Discuss",
            "Vote totals are shown. Talk the cards through, comment on them and merge any that match.",
        ],
        ["actions", "Actions", "Agree what to do next and add action items. The cards can no longer be changed."],
        ["end", "End", "The retro has ended, so the board can only be read."],
    ] as const)("highlights %s and says what it is for", (stage, label, purpose) => {
        // when
        renderWithProviders(<StageBar board={makeBoardView({ stage, cards_hidden: stage === "write" })} />);

        // then
        expect(currentStep()).toContain(label);
        expect(within(screen.getByRole("region", { name: "Retro stage" })).getByText(purpose)).toBeInTheDocument();
    });

    it("follows the board when its stage moves on", () => {
        // given
        const { rerender } = renderWithProviders(<StageBar board={makeBoardView({ stage: "vote" })} />);

        // when
        rerender(<StageBar board={makeBoardView({ stage: "discuss", cards_hidden: false })} />);

        // then
        expect(currentStep()).toContain("Discuss");
    });
});
