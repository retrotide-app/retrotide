import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { MarketingSection } from "./MarketingSection";

describe("MarketingSection", () => {
    it("names its region after its heading and holds its intro and content", () => {
        // given
        const ui = (
            <MarketingSection title="Hidden cards" intro="Nobody reads them early.">
                <p>Body copy</p>
            </MarketingSection>
        );

        // when
        renderWithProviders(ui);

        // then
        const section = screen.getByRole("region", { name: "Hidden cards" });
        expect(section).toContainElement(screen.getByRole("heading", { level: 2, name: "Hidden cards" }));
        expect(section).toContainElement(screen.getByText("Nobody reads them early."));
        expect(section).toContainElement(screen.getByText("Body copy"));
    });
});
