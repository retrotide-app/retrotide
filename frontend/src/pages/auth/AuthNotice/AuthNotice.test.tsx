import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../../test-utils/render";
import { AuthNotice } from "./AuthNotice";

describe("AuthNotice", () => {
    it("announces the outcome in a status region", () => {
        // given
        const ui = <AuthNotice>Your email address is verified.</AuthNotice>;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Your email address is verified.");
    });

    it("takes focus when it appears, so a keyboard user is not left on a button that has gone", () => {
        // given
        const ui = <AuthNotice>Link sent. Check your inbox.</AuthNotice>;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("status")).toHaveFocus();
    });
});
