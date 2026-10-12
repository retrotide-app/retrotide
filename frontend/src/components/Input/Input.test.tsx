import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Input } from "./Input";

describe("Input", () => {
    it("is reachable through the label that points at it", () => {
        // given
        const markup = (
            <>
                <label htmlFor="board-name">Board name</label>
                <Input id="board-name" />
            </>
        );

        // when
        renderWithProviders(markup);

        // then
        expect(screen.getByLabelText("Board name")).toBeInstanceOf(HTMLInputElement);
    });

    it("reports every keystroke through its change handler", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Input placeholder="Board name" onChange={onChange} />);

        // when
        await user.type(screen.getByPlaceholderText("Board name"), "wins");

        // then
        expect(onChange).toHaveBeenCalledTimes(4);
        expect(screen.getByPlaceholderText("Board name")).toHaveValue("wins");
    });

    it("shows the value it is controlled with and does not drift when typed into", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Input placeholder="Board name" value="sprint 12" onChange={onChange} />);

        // when
        await user.type(screen.getByPlaceholderText("Board name"), "x");

        // then
        expect(onChange).toHaveBeenCalledOnce();
        expect(screen.getByPlaceholderText("Board name")).toHaveValue("sprint 12");
    });

    it("refuses input while it is disabled", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Input placeholder="Board name" disabled onChange={onChange} />);

        // when
        await user.type(screen.getByPlaceholderText("Board name"), "wins");

        // then
        expect(screen.getByPlaceholderText("Board name")).toBeDisabled();
        expect(onChange).not.toHaveBeenCalled();
    });

    it("refuses input while it is read only", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<Input placeholder="Board name" readOnly value="locked" onChange={onChange} />);

        // when
        await user.type(screen.getByPlaceholderText("Board name"), "wins");

        // then
        expect(onChange).not.toHaveBeenCalled();
        expect(screen.getByPlaceholderText("Board name")).toHaveValue("locked");
    });

    it("forwards native validation attributes to the underlying input", () => {
        // given
        const markup = <Input type="email" required maxLength={40} autoComplete="off" placeholder="Email" />;

        // when
        renderWithProviders(markup);

        // then
        const input = screen.getByPlaceholderText("Email");
        expect(input).toHaveAttribute("type", "email");
        expect(input).toBeRequired();
        expect(input).toHaveAttribute("maxlength", "40");
        expect(input).toHaveAttribute("autocomplete", "off");
    });

    it("marks itself as invalid when it is described by an error message", () => {
        // given
        const markup = (
            <>
                <Input placeholder="Email" aria-invalid aria-describedby="email-error" />
                <span id="email-error">That address is not allowed</span>
            </>
        );

        // when
        renderWithProviders(markup);

        // then
        const input = screen.getByPlaceholderText("Email");
        expect(input).toHaveAttribute("aria-invalid", "true");
        expect(input).toHaveAccessibleDescription("That address is not allowed");
    });

    it("lets the browser pick the text direction from what is typed", () => {
        // given
        const markup = <Input placeholder="Board name" />;

        // when
        renderWithProviders(markup);

        // then
        expect(screen.getByPlaceholderText("Board name")).toHaveAttribute("dir", "auto");
    });

    it("keeps a caller supplied class alongside its own classes", () => {
        // given
        const className = "search-field";

        // when
        renderWithProviders(<Input placeholder="Search" className={className} />);

        // then
        const input = screen.getByPlaceholderText("Search");
        expect(input).toHaveClass(className);
        expect(input.className.split(" ").length).toBeGreaterThan(1);
    });

    it("styles itself differently once it is asked to fill the width", () => {
        // given
        const markup = (
            <>
                <Input placeholder="Narrow" />
                <Input placeholder="Wide" fullWidth />
            </>
        );

        // when
        renderWithProviders(markup);

        // then
        const narrow = screen.getByPlaceholderText("Narrow");
        const wide = screen.getByPlaceholderText("Wide");
        expect(wide.className).not.toBe(narrow.className);
        expect(wide.className.split(" ").length).toBe(narrow.className.split(" ").length + 1);
    });

    it("does not leak the fullWidth flag onto the DOM element", () => {
        // given
        const markup = <Input placeholder="Wide" fullWidth />;

        // when
        renderWithProviders(markup);

        // then
        expect(screen.getByPlaceholderText("Wide")).not.toHaveAttribute("fullwidth");
    });
});
