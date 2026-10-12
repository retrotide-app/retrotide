import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { SettingsRow } from "./SettingsRow";

describe("SettingsRow", () => {
    it("names its region after its heading and holds its control", () => {
        // when
        renderWithProviders(
            <SettingsRow title="Display name" headingLevel={2}>
                <button type="button">Save</button>
            </SettingsRow>,
        );

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Display name" })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: "Display name" })).toContainElement(
            screen.getByRole("button", { name: "Save" }),
        );
    });

    it("heads itself at the level it is given", () => {
        // when
        renderWithProviders(<SettingsRow title="Guest access" headingLevel={3} />);

        // then
        expect(screen.getByRole("heading", { level: 3, name: "Guest access" })).toBeInTheDocument();
        expect(screen.queryByRole("heading", { level: 2 })).not.toBeInTheDocument();
    });

    it("explains itself when given a description", () => {
        // when
        renderWithProviders(
            <SettingsRow title="Password" headingLevel={2} description="Change the password you sign in with." />,
        );

        // then
        expect(screen.getByRole("region", { name: "Password" })).toHaveTextContent(
            "Change the password you sign in with.",
        );
    });
});
