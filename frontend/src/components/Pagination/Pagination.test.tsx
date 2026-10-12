import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Pagination } from "./Pagination";

describe("Pagination", () => {
    it("stays away when everything fits on one page", () => {
        // given
        const onChange = vi.fn();

        // when
        renderWithProviders(<Pagination label="User pages" offset={0} limit={20} total={20} onChange={onChange} />);

        // then
        expect(screen.queryByRole("navigation", { name: "User pages" })).not.toBeInTheDocument();
    });

    it("says which rows are showing", () => {
        // given
        const onChange = vi.fn();

        // when
        renderWithProviders(<Pagination label="User pages" offset={20} limit={20} total={45} onChange={onChange} />);

        // then
        expect(screen.getByRole("navigation", { name: "User pages" })).toHaveTextContent("21 to 40 of 45");
    });

    it("moves to the next page", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Pagination label="User pages" offset={0} limit={20} total={45} onChange={onChange} />);

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(onChange).toHaveBeenCalledWith(20);
    });

    it("moves to the previous page", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Pagination label="User pages" offset={40} limit={20} total={45} onChange={onChange} />);

        // when
        await user.click(screen.getByRole("button", { name: "Previous" }));

        // then
        expect(onChange).toHaveBeenCalledWith(20);
    });

    it("holds Previous on the first page", () => {
        // given
        const onChange = vi.fn();

        // when
        renderWithProviders(<Pagination label="User pages" offset={0} limit={20} total={45} onChange={onChange} />);

        // then
        expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Next" })).toBeEnabled();
    });

    it("holds Next on the last page", () => {
        // given
        const onChange = vi.fn();

        // when
        renderWithProviders(<Pagination label="User pages" offset={40} limit={20} total={45} onChange={onChange} />);

        // then
        expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
        expect(screen.getByText("41 to 45 of 45")).toBeInTheDocument();
    });

    it("still lets a reader step back from a page past the end", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Pagination label="User pages" offset={40} limit={20} total={0} onChange={onChange} />);

        // when
        await user.click(screen.getByRole("button", { name: "Previous" }));

        // then
        expect(onChange).toHaveBeenCalledWith(20);
    });
});
