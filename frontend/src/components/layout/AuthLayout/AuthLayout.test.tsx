import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { AuthLayout } from "./AuthLayout";

describe("AuthLayout", () => {
    it("puts the page title in the one main heading", () => {
        // given
        const ui = <AuthLayout title="Sign in">form</AuthLayout>;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Sign in" })).toBeInTheDocument();
        expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    });

    it("draws its content inside the main landmark, which the skip link can focus", () => {
        // given
        const ui = (
            <AuthLayout title="Sign in">
                <p>the sign in form</p>
            </AuthLayout>
        );

        // when
        renderWithProviders(ui);

        // then
        const main = screen.getByRole("main");
        expect(main).toContainElement(screen.getByText("the sign in form"));
        expect(main).toHaveAttribute("id", "main-content");
        expect(main).toHaveAttribute("tabindex", "-1");
    });

    it("links the logo home under the site's name", () => {
        // given
        const siteInfo = { site_name: "Harbour Retros" };

        // when
        renderWithProviders(<AuthLayout title="Sign in">form</AuthLayout>, { siteInfo });

        // then
        expect(screen.getByRole("link", { name: "Harbour Retros" })).toHaveAttribute("href", "/");
    });

    it("draws the footer below the card", () => {
        // given
        const ui = (
            <AuthLayout title="Sign in" footer={<p>No account yet?</p>}>
                form
            </AuthLayout>
        );

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("main")).toContainElement(screen.getByText("No account yet?"));
    });

    it("draws no app navigation, so the form stands on its own", () => {
        // given
        const ui = <AuthLayout title="Create an account">form</AuthLayout>;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.queryByRole("navigation")).not.toBeInTheDocument();
        expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    });
});
