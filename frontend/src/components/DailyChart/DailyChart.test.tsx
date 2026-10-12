import { fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { DailyChart } from "./DailyChart";

const points = [
    { date: "2026-10-03", value: 2 },
    { date: "2026-10-04", value: 5 },
    { date: "2026-10-05", value: 1 },
];

const noun = { one: "account", other: "accounts" };

function renderChart() {
    return renderWithProviders(<DailyChart title="New accounts" headingLevel={3} points={points} noun={noun} />);
}

describe("DailyChart", () => {
    it("heads the chart and gives the period's total", () => {
        // when
        renderChart();

        // then
        const chart = screen.getByRole("region", { name: "New accounts" });
        expect(within(chart).getByRole("heading", { level: 3, name: "New accounts" })).toBeInTheDocument();
        expect(within(chart).getByText("8")).toBeInTheDocument();
    });

    it("sums up the series in words for anyone who cannot see the plot", () => {
        // when
        renderChart();

        // then
        const plot = screen.getByRole("group", { name: "New accounts, day by day" });
        expect(plot).toHaveAccessibleDescription(
            "8 accounts in the last 3 days. Busiest day: Sun 4 October, with 5. Use the left and right arrow keys to read each day.",
        );
    });

    it("reads out the latest day when the plot takes focus", async () => {
        // given
        const user = userEvent.setup();
        renderChart();

        // when
        await user.tab();

        // then
        expect(screen.getByRole("group", { name: "New accounts, day by day" })).toHaveFocus();
        expect(screen.getByText("Mon 5 October: 1 account")).toBeInTheDocument();
    });

    it("steps through the days with the arrow keys", async () => {
        // given
        const user = userEvent.setup();
        renderChart();
        await user.tab();

        // when
        await user.keyboard("{ArrowLeft}");

        // then
        expect(screen.getByText("Sun 4 October: 5 accounts")).toBeInTheDocument();
    });

    it("jumps to the first and the last day", async () => {
        // given
        const user = userEvent.setup();
        renderChart();
        await user.tab();

        // when
        await user.keyboard("{Home}");

        // then
        expect(screen.getByText("Sat 3 October: 2 accounts")).toBeInTheDocument();

        // when
        await user.keyboard("{End}");

        // then
        expect(screen.getByText("Mon 5 October: 1 account")).toBeInTheDocument();
    });

    it("stops at the edges of the period", async () => {
        // given
        const user = userEvent.setup();
        renderChart();
        await user.tab();

        // when
        await user.keyboard("{ArrowRight}");

        // then
        expect(screen.getByText("Mon 5 October: 1 account")).toBeInTheDocument();
    });

    it("clears the readout when focus leaves", async () => {
        // given
        const user = userEvent.setup();
        renderChart();
        await user.tab();

        // when
        await user.tab();

        // then
        expect(screen.queryByText("Mon 5 October: 1 account")).not.toBeInTheDocument();
    });

    it("shows the hovered day in the tooltip without reading it out", () => {
        // given
        renderChart();
        const plot = screen.getByRole("group", { name: "New accounts, day by day" });
        vi.spyOn(plot, "getBoundingClientRect").mockReturnValue(
            DOMRect.fromRect({ x: 0, y: 0, width: 300, height: 100 }),
        );

        // when
        fireEvent.pointerMove(plot, { clientX: 150 });

        // then
        expect(screen.getAllByText("Sun 4 October")).toHaveLength(2);
        expect(screen.queryByText("Sun 4 October: 5 accounts")).not.toBeInTheDocument();
    });

    it("lists every day in a table", async () => {
        // given
        const user = userEvent.setup();
        renderChart();

        // when
        await user.click(screen.getByText("Show the numbers"));

        // then
        const table = screen.getByRole("table", { name: "New accounts per day" });
        const rows = within(table).getAllByRole("row");
        expect(rows).toHaveLength(4);
        expect(rows[2]).toHaveTextContent("Sun 4 October");
        expect(rows[2]).toHaveTextContent("5");
    });
});
