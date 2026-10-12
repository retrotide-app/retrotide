import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeBoardTemplate } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { TemplateCard } from "./TemplateCard";

const sailboat = makeBoardTemplate({
    key: "sailboat",
    name: "Sailboat",
    description: "Look at what drives the team and what holds it back.",
    columns: [
        { title: "Wind", description: "What pushed us forward?", colour: "green" },
        { title: "Anchors", description: "What held us back?", colour: "red" },
        { title: "Rocks", description: "What risks lie ahead?", colour: "amber" },
        { title: "Island", description: "Where are we heading?", colour: "blue" },
    ],
});

describe("TemplateCard", () => {
    it("links the template's name to its page", () => {
        // given
        const ui = <TemplateCard template={sailboat} href="/templates/sailboat" />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("heading", { level: 3, name: "Sailboat" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Sailboat" })).toHaveAttribute("href", "/templates/sailboat");
    });

    it("shows the description and how many columns the template has", () => {
        // given
        const ui = <TemplateCard template={sailboat} href="/templates/sailboat" />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByText("Look at what drives the team and what holds it back.")).toBeInTheDocument();
        expect(screen.getByText("4 columns")).toBeInTheDocument();
    });
});
