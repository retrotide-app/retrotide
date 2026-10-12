import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { makeOrganisation } from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import { DeletedOrganisation } from "./DeletedOrganisation";

const deletedAt = "2026-09-20T12:00:00Z";

beforeEach(() => {
    document.title = "";
});

describe("DeletedOrganisation", () => {
    it("tells the owner until when the server says they can restore it, and takes them to the settings", () => {
        // given
        const harbour = makeOrganisation({
            slug: "harbour",
            role: "owner",
            deleted_at: deletedAt,
            restorable_until: "2026-10-18T12:00:00Z",
        });

        // when
        renderWithProviders(<DeletedOrganisation organisation={harbour} pageTitle="Teams" />);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Harbour is deleted" })).toBeInTheDocument();
        expect(screen.getByText(/You can restore it until 18 October 2026/)).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Go to organisation settings" })).toHaveAttribute(
            "href",
            "/o/harbour/settings",
        );
        expect(document.title).toContain("Teams");
    });

    it("names no deadline when the answer carries none", () => {
        // given
        const harbour = makeOrganisation({ role: "owner", deleted_at: deletedAt });

        // when
        renderWithProviders(<DeletedOrganisation organisation={harbour} pageTitle="Members" />);

        // then
        expect(screen.getByText("You can restore it from the organisation settings.")).toBeInTheDocument();
    });
});
