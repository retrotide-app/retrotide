import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { SectionNav } from "./SectionNav";

const items = [
    { id: "general", label: "General", to: "/admin/settings?section=general" },
    { id: "email", label: "Email", to: "/admin/settings?section=email" },
];

describe("SectionNav", () => {
    it("lists every section as a link in a named navigation", () => {
        // when
        renderWithProviders(<SectionNav label="Settings sections" items={items} current="general" />);

        // then
        const nav = screen.getByRole("navigation", { name: "Settings sections" });
        expect(
            within(nav)
                .getAllByRole("link")
                .map(link => link.textContent),
        ).toEqual(["General", "Email"]);
        expect(within(nav).getByRole("link", { name: "Email" })).toHaveAttribute(
            "href",
            "/admin/settings?section=email",
        );
    });

    it("marks only the current section", () => {
        // when
        renderWithProviders(<SectionNav label="Settings sections" items={items} current="email" />);

        // then
        expect(screen.getByRole("link", { name: "Email" })).toHaveAttribute("aria-current", "page");
        expect(screen.getByRole("link", { name: "General" })).not.toHaveAttribute("aria-current");
    });
});
