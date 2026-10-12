import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { ToggleGroup } from "./ToggleGroup";

const OPTIONS: ReadonlyArray<{ value: "grid" | "list"; label: string }> = [
    { value: "grid", label: "Grid" },
    { value: "list", label: "List" },
];

describe("ToggleGroup", () => {
    it("names the group and marks only the chosen option as pressed", () => {
        // when
        renderWithProviders(<ToggleGroup label="View" options={OPTIONS} value="list" onChange={vi.fn()} />);

        // then
        const group = screen.getByRole("group", { name: "View" });
        expect(within(group).getByRole("button", { name: "Grid" })).toHaveAttribute("aria-pressed", "false");
        expect(within(group).getByRole("button", { name: "List" })).toHaveAttribute("aria-pressed", "true");
    });

    it("hands back the option that was picked", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<ToggleGroup label="View" options={OPTIONS} value="grid" onChange={onChange} />);

        // when
        await user.click(screen.getByRole("button", { name: "List" }));

        // then
        expect(onChange).toHaveBeenCalledExactlyOnceWith("list");
    });
});
