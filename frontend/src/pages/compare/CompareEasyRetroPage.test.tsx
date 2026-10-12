import { screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { CompareEasyRetroPage } from "./CompareEasyRetroPage";

beforeEach(() => {
    document.title = "";
});

describe("CompareEasyRetroPage", () => {
    it("heads the page and titles the document", () => {
        // given
        const ui = <CompareEasyRetroPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(
            screen.getByRole("heading", { level: 1, name: "RetroTide compared with EasyRetro" }),
        ).toBeInTheDocument();
        expect(document.title).toContain("RetroTide compared with EasyRetro");
    });

    it("says when the comparison was checked", () => {
        // given
        const ui = <CompareEasyRetroPage />;

        // when
        renderWithProviders(ui);

        // then
        const hero = within(screen.getByRole("region", { name: "RetroTide compared with EasyRetro" }));
        expect(hero.getByText("Checked in October 2026")).toHaveAttribute("datetime", "2026-10");
    });

    it("states that no facilitator, owner included, can read hidden cards before the reveal", () => {
        // given
        const ui = <CompareEasyRetroPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(
            screen.getByText(/No RetroTide facilitator, owner included, can read hidden cards before the reveal\./),
        ).toBeInTheDocument();
    });

    it("compares free boards, action carry-over and AI in one table", () => {
        // given
        const ui = <CompareEasyRetroPage />;

        // when
        renderWithProviders(ui);

        // then
        const table = within(screen.getByRole("table"));
        const boards = within(table.getByRole("row", { name: /^Boards on the free plan/ }));
        const actions = within(table.getByRole("row", { name: /^Unfinished actions carried/ }));
        const ai = within(table.getByRole("row", { name: /^AI template generator/ }));
        expect(boards.getByText("Unlimited boards for one team")).toBeInTheDocument();
        expect(boards.getByText("1 public board a month")).toBeInTheDocument();
        expect(actions.getByText("Free on every plan")).toBeInTheDocument();
        expect(ai.getByText("No AI features, by choice")).toBeInTheDocument();
        expect(ai.getByText("Works on its Free plan")).toBeInTheDocument();
    });

    it("describes action carry-over as free, never as something EasyRetro lacks", () => {
        // given
        const ui = <CompareEasyRetroPage />;

        // when
        renderWithProviders(ui);

        // then
        expect(screen.getByRole("heading", { level: 3, name: "Action carry-over is free" })).toBeInTheDocument();
        expect(screen.queryByText(/easyretro (lacks|has no|doesn't|does not)/i)).not.toBeInTheDocument();
    });
});
