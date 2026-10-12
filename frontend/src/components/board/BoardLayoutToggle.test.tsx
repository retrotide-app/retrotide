import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { BoardLayoutToggle } from "./BoardLayoutToggle";

describe("BoardLayoutToggle", () => {
    it("marks the layout in use as pressed", () => {
        // when
        renderWithProviders(<BoardLayoutToggle layout="list" onChange={vi.fn()} />);

        // then
        const group = screen.getByRole("group", { name: "Board layout" });
        expect(group).toContainElement(screen.getByRole("button", { name: "List", pressed: true }));
        expect(screen.getByRole("button", { name: "Columns", pressed: false })).toBeInTheDocument();
    });

    it("asks for the layout the viewer picks", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<BoardLayoutToggle layout="columns" onChange={onChange} />);

        // when
        await user.click(screen.getByRole("button", { name: "List" }));

        // then
        expect(onChange).toHaveBeenCalledExactlyOnceWith("list");
    });
});
