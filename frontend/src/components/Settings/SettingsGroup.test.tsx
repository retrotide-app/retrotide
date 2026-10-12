import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { SettingsGroup } from "./SettingsGroup";
import { SettingsRow } from "./SettingsRow";

describe("SettingsGroup", () => {
    it("holds its rows", () => {
        // when
        renderWithProviders(
            <SettingsGroup>
                <SettingsRow title="Display name" headingLevel={2} />
                <SettingsRow title="Password" headingLevel={2} />
            </SettingsGroup>,
        );

        // then
        expect(screen.getAllByRole("region")).toHaveLength(2);
        expect(screen.getByRole("region", { name: "Display name" })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Password" })).toBeInTheDocument();
    });

    it("heads the group as a region of its own when given a title", () => {
        // when
        renderWithProviders(
            <SettingsGroup title="Danger zone">
                <SettingsRow title="Delete organisation" headingLevel={3} />
            </SettingsGroup>,
        );

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Danger zone" })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Danger zone" })).toContainElement(
            screen.getByRole("region", { name: "Delete organisation" }),
        );
    });
});
