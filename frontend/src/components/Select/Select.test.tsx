import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Select } from "./Select";

function options() {
    return (
        <>
            <option value="votes">Votes</option>
            <option value="date">Date</option>
            <option value="random">Random</option>
        </>
    );
}

describe("Select", () => {
    it("renders every option it is given", () => {
        // given
        const markup = <Select aria-label="Sort cards by">{options()}</Select>;

        // when
        renderWithProviders(markup);

        // then
        const rendered = screen.getAllByRole("option");
        expect(rendered).toHaveLength(3);
        expect(rendered.map(option => option.textContent)).toEqual(["Votes", "Date", "Random"]);
    });

    it("is reachable through the label that points at it", () => {
        // given
        const markup = (
            <>
                <label htmlFor="sort-order">Sort order</label>
                <Select id="sort-order">{options()}</Select>
            </>
        );

        // when
        renderWithProviders(markup);

        // then
        expect(screen.getByLabelText("Sort order")).toBeInstanceOf(HTMLSelectElement);
    });

    it("reports the option that was chosen", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <Select aria-label="Sort cards by" defaultValue="votes" onChange={onChange}>
                {options()}
            </Select>,
        );

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Sort cards by" }), "random");

        // then
        expect(onChange).toHaveBeenCalledOnce();
        expect(screen.getByRole("combobox", { name: "Sort cards by" })).toHaveValue("random");
    });

    it("shows the value it is controlled with as the current selection", () => {
        // given
        const onChange = vi.fn();

        // when
        renderWithProviders(
            <Select aria-label="Sort cards by" value="date" onChange={onChange}>
                {options()}
            </Select>,
        );

        // then
        expect(screen.getByRole("combobox", { name: "Sort cards by" })).toHaveValue("date");
        expect(screen.getByRole<HTMLOptionElement>("option", { name: "Date" }).selected).toBe(true);
    });

    it("cannot be changed while it is disabled", () => {
        // given
        const onChange = vi.fn();

        // when
        renderWithProviders(
            <Select aria-label="Sort cards by" disabled defaultValue="votes" onChange={onChange}>
                {options()}
            </Select>,
        );

        // then
        expect(screen.getByRole("combobox", { name: "Sort cards by" })).toBeDisabled();
        expect(onChange).not.toHaveBeenCalled();
    });

    it("forwards native attributes to the underlying select", () => {
        // given
        const markup = (
            <Select aria-label="Sort cards by" name="sort" required>
                {options()}
            </Select>
        );

        // when
        renderWithProviders(markup);

        // then
        const select = screen.getByRole("combobox", { name: "Sort cards by" });
        expect(select).toHaveAttribute("name", "sort");
        expect(select).toBeRequired();
    });

    it("defaults to the medium size and gives the small size its own styling", () => {
        // given
        const selects = (
            <>
                <Select aria-label="fallback">{options()}</Select>
                <Select aria-label="medium" size="medium">
                    {options()}
                </Select>
                <Select aria-label="small" size="small">
                    {options()}
                </Select>
            </>
        );

        // when
        renderWithProviders(selects);

        // then
        const fallback = screen.getByRole("combobox", { name: "fallback" });
        expect(fallback.className).toBe(screen.getByRole("combobox", { name: "medium" }).className);
        expect(fallback.className).not.toBe(screen.getByRole("combobox", { name: "small" }).className);
    });

    it("never passes its size on as the native visible row count", () => {
        // given
        const markup = (
            <Select aria-label="Sort cards by" size="small">
                {options()}
            </Select>
        );

        // when
        renderWithProviders(markup);

        // then
        expect(screen.getByRole("combobox", { name: "Sort cards by" })).not.toHaveAttribute("size");
    });

    it("keeps a caller supplied class alongside its own classes", () => {
        // given
        const className = "compact-select";

        // when
        renderWithProviders(
            <Select aria-label="Sort cards by" className={className}>
                {options()}
            </Select>,
        );

        // then
        const select = screen.getByRole("combobox", { name: "Sort cards by" });
        expect(select).toHaveClass(className);
        expect(select.className.split(" ").length).toBeGreaterThan(1);
    });
});
