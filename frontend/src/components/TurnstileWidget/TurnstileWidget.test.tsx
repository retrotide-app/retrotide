import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { makeTurnstileControl } from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import { TurnstileWidget } from "./TurnstileWidget";

describe("TurnstileWidget", () => {
    it("draws nothing while the site has Turnstile switched off", () => {
        // given
        const off = makeTurnstileControl({ enabled: false });

        // when
        const { container } = renderWithProviders(<TurnstileWidget control={off} />);

        // then
        expect(container).toBeEmptyDOMElement();
        expect(off.attach).not.toHaveBeenCalled();
    });

    it("hands its container to the control so the widget can be mounted into it", () => {
        // given
        const on = makeTurnstileControl({ enabled: true });

        // when
        renderWithProviders(<TurnstileWidget control={on} />);

        // then
        expect(on.attach).toHaveBeenCalledWith(expect.any(HTMLDivElement));
    });

    it("shows why the check cannot run", () => {
        // given
        const failed = makeTurnstileControl({ enabled: true, error: "The verification check could not load." });

        // when
        renderWithProviders(<TurnstileWidget control={failed} />);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("The verification check could not load.");
    });

    it("shows no alert while the check is fine", () => {
        // given
        const fine = makeTurnstileControl({ enabled: true, token: "token-1" });

        // when
        renderWithProviders(<TurnstileWidget control={fine} />);

        // then
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
});
