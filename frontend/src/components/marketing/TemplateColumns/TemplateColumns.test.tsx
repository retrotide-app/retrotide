import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import type { TemplateColumnResponse } from "../../../types/api";
import { TemplateColumns } from "./TemplateColumns";

const columns: readonly TemplateColumnResponse[] = [
    { title: "Wind", description: "What pushed us forward?", colour: "green" },
    { title: "Anchors", description: "What held us back?", colour: "red" },
];

describe("TemplateColumns", () => {
    it("lists every column in order with its title and description", () => {
        // given
        const ui = <TemplateColumns columns={columns} label="Sailboat columns" />;

        // when
        renderWithProviders(ui);

        // then
        const list = within(screen.getByRole("list", { name: "Sailboat columns" }));
        expect(list.getAllByRole("heading", { level: 3 }).map(heading => heading.textContent)).toEqual([
            "Wind",
            "Anchors",
        ]);
        expect(list.getByText("What pushed us forward?")).toBeInTheDocument();
        expect(list.getByText("What held us back?")).toBeInTheDocument();
    });
});
