import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Logo } from "./Logo";

describe("Logo", () => {
    it("is one image named by its label, whichever theme is showing", () => {
        // given
        const label = "RetroTide";

        // when
        renderWithProviders(<Logo label={label} />);

        // then
        expect(screen.getAllByRole("img")).toHaveLength(1);
        expect(screen.getByRole("img", { name: label })).toBeInTheDocument();
    });

    it("carries both the light and the dark version of the logo", () => {
        // given
        const ui = <Logo label="RetroTide" />;

        // when
        const { container } = renderWithProviders(ui);

        // then
        const sources = Array.from(container.querySelectorAll("img")).map(image => image.getAttribute("src"));
        expect(sources).toEqual(["/logo.png", "/logo-dark.png"]);
    });

    it("stays out of the accessibility tree when it has no label", () => {
        // given
        const ui = <Logo />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.queryByRole("img")).not.toBeInTheDocument();
    });

    it("keeps a caller supplied class alongside its own", () => {
        // given
        const className = "auth-logo";

        // when
        renderWithProviders(<Logo label="RetroTide" className={className} />);

        // then
        expect(screen.getByRole("img", { name: "RetroTide" })).toHaveClass(className);
    });
});
