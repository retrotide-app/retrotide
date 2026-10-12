import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { builtInOption, savedOption, type TemplateOption, type TemplateOptionGroup } from "../../domain/savedTemplates";
import { makeBoardTemplate, makeSavedTemplate } from "../../test-utils/fixtures";
import { TemplatePicker } from "./TemplatePicker";

const startStopContinue = builtInOption(
    makeBoardTemplate({
        key: "start-stop-continue",
        name: "Start, Stop, Continue",
        description: "Decide what to begin, what to drop and what to keep.",
        columns: [
            { title: "Start", description: "What should we begin doing?", colour: "green" },
            { title: "Stop", description: "What should we stop doing?", colour: "red" },
            { title: "Continue", description: "What should we keep doing?", colour: "blue" },
        ],
    }),
);
const blank = builtInOption(makeBoardTemplate());
const ours = savedOption(
    makeSavedTemplate({
        id: "s1",
        name: "Our retro",
        description: "Saved from the sprint review board.",
        scope: "team",
        team_name: "Platform",
        columns: [{ title: "Went well", description: "What should we keep doing?", colour: "green" }],
    }),
);

function builtIn(...options: TemplateOption[]): TemplateOptionGroup {
    return { label: "Built-in templates", options };
}

describe("TemplatePicker", () => {
    it("offers every template as one choice in a named group, with the picked one checked", () => {
        // when
        render(
            <TemplatePicker
                legend="Template"
                groups={[builtIn(startStopContinue, blank)]}
                selected={blank.value}
                onSelect={vi.fn()}
            />,
        );

        // then
        expect(screen.getByRole("group", { name: "Template" })).toBeInTheDocument();
        expect(screen.getByRole("radio", { name: "Start, Stop, Continue" })).not.toBeChecked();
        expect(screen.getByRole("radio", { name: "Blank" })).toBeChecked();
        expect(screen.queryByText("Built-in templates")).not.toBeInTheDocument();
    });

    it("describes each template with its columns", () => {
        // when
        render(
            <TemplatePicker legend="Template" groups={[builtIn(startStopContinue)]} selected="" onSelect={vi.fn()} />,
        );

        // then
        expect(screen.getByRole("radio", { name: "Start, Stop, Continue" })).toHaveAccessibleDescription(
            "Decide what to begin, what to drop and what to keep. Columns: Start, Stop, Continue.",
        );
    });

    it("labels your templates above the built-in ones and says who a saved one is for", () => {
        // when
        render(
            <TemplatePicker
                legend="Template"
                groups={[{ label: "Your templates", options: [ours] }, builtIn(blank)]}
                selected={ours.value}
                onSelect={vi.fn()}
            />,
        );

        // then
        const yours = screen.getByRole("group", { name: "Your templates" });
        expect(within(yours).getByRole("radio", { name: "Our retro" })).toBeChecked();
        expect(within(yours).getByRole("radio", { name: "Our retro" })).toHaveAccessibleDescription(
            "Shared with Platform Saved from the sprint review board. Columns: Went well.",
        );
        expect(
            within(screen.getByRole("group", { name: "Built-in templates" })).getByRole("radio", { name: "Blank" }),
        ).not.toBeChecked();
    });

    it("hands back the value of the template picked", async () => {
        // given
        const onSelect = vi.fn();
        const user = userEvent.setup();
        render(
            <TemplatePicker
                legend="Template"
                groups={[{ label: "Your templates", options: [ours] }, builtIn(startStopContinue, blank)]}
                selected={startStopContinue.value}
                onSelect={onSelect}
            />,
        );

        // when
        await user.click(screen.getByRole("radio", { name: "Our retro" }));

        // then
        expect(onSelect).toHaveBeenCalledExactlyOnceWith("saved:s1");
    });

    it("ties a refusal to the group", () => {
        // when
        render(
            <TemplatePicker
                legend="Template"
                groups={[builtIn(blank)]}
                selected={blank.value}
                onSelect={vi.fn()}
                error="unknown template"
            />,
        );

        // then
        expect(screen.getByRole("group", { name: "Template" })).toHaveAccessibleDescription("unknown template");
    });
});
