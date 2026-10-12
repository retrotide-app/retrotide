import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StepProgress } from "./StepProgress";

describe("StepProgress", () => {
    it("lists every step in order, each saying where it sits", () => {
        // when
        render(<StepProgress label="Progress" steps={["Team", "Template", "Details"]} current={1} />);

        // then
        const steps = within(screen.getByRole("list", { name: "Progress" })).getAllByRole("listitem");
        expect(steps.map(step => step.textContent)).toEqual([
            "1Step 1 of 3: Team",
            "2Step 2 of 3: Template",
            "3Step 3 of 3: Details",
        ]);
    });

    it("lists the same steps and marks the same one when compact", () => {
        // when
        render(<StepProgress label="Stages" steps={["Write", "Vote", "Discuss"]} current={2} compact />);

        // then
        const steps = within(screen.getByRole("list", { name: "Stages" })).getAllByRole("listitem");
        expect(steps.map(step => step.textContent)).toEqual([
            "1Step 1 of 3: Write",
            "2Step 2 of 3: Vote",
            "3Step 3 of 3: Discuss",
        ]);
        expect(steps.map(step => step.getAttribute("aria-current"))).toEqual([null, null, "step"]);
    });

    it("marks only the current step", () => {
        // when
        render(<StepProgress label="Progress" steps={["Team", "Template", "Details"]} current={1} />);

        // then
        const steps = screen.getAllByRole("listitem");
        expect(steps.map(step => step.getAttribute("aria-current"))).toEqual([null, "step", null]);
    });
});
