import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { Button } from "../../../components/Button/Button";
import { renderWithProviders } from "../../../test-utils/render";
import { AdminPage } from "./AdminPage";

beforeEach(() => {
    document.title = "";
});

describe("AdminPage", () => {
    it("heads the page and names the tab after it", () => {
        // given
        const title = "Settings";

        // when
        renderWithProviders(
            <AdminPage title={title}>
                <p>the page body</p>
            </AdminPage>,
            { siteInfo: { site_name: "RetroTide" } },
        );

        // then
        expect(screen.getByRole("heading", { level: 2, name: title })).toBeInTheDocument();
        expect(screen.getByRole("region", { name: title })).toHaveTextContent("the page body");
        expect(document.title).toContain("Settings");
        expect(document.title).toContain("RetroTide");
    });

    it("puts the page's actions beside its heading", () => {
        // given
        const actions = <Button type="button">Send test email</Button>;

        // when
        renderWithProviders(<AdminPage title="Settings" actions={actions} />);

        // then
        expect(screen.getByRole("region", { name: "Settings" })).toContainElement(
            screen.getByRole("button", { name: "Send test email" }),
        );
    });
});
