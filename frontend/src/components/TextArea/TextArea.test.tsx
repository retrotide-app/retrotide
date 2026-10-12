import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { TextArea } from "./TextArea";

describe("TextArea", () => {
    it("is reachable through the label that points at it", () => {
        // given
        const markup = (
            <>
                <label htmlFor="card-text">Card text</label>
                <TextArea id="card-text" />
            </>
        );

        // when
        renderWithProviders(markup);

        // then
        expect(screen.getByLabelText("Card text")).toBeInstanceOf(HTMLTextAreaElement);
    });

    it("collects the text that is typed into it", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<TextArea placeholder="Say something" onChange={onChange} />);

        // when
        await user.type(screen.getByPlaceholderText("Say something"), "went well");

        // then
        expect(screen.getByPlaceholderText("Say something")).toHaveValue("went well");
        expect(onChange).toHaveBeenCalledTimes("went well".length);
    });

    it("keeps newlines that are typed into it", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<TextArea placeholder="Say something" />);

        // when
        await user.type(screen.getByPlaceholderText("Say something"), "went well{enter}could improve");

        // then
        expect(screen.getByPlaceholderText("Say something")).toHaveValue("went well\ncould improve");
    });

    it("shows the value it is controlled with", () => {
        // given
        const onChange = vi.fn();

        // when
        renderWithProviders(<TextArea placeholder="Say something" value="ship it" onChange={onChange} />);

        // then
        expect(screen.getByPlaceholderText("Say something")).toHaveValue("ship it");
    });

    it("refuses input while it is disabled", async () => {
        // given
        const onChange = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<TextArea placeholder="Say something" disabled onChange={onChange} />);

        // when
        await user.type(screen.getByPlaceholderText("Say something"), "went well");

        // then
        expect(screen.getByPlaceholderText("Say something")).toBeDisabled();
        expect(onChange).not.toHaveBeenCalled();
    });

    it("forwards native attributes to the underlying textarea", () => {
        // given
        const markup = <TextArea placeholder="Say something" rows={7} maxLength={300} required name="body" />;

        // when
        renderWithProviders(markup);

        // then
        const textarea = screen.getByPlaceholderText("Say something");
        expect(textarea).toHaveAttribute("rows", "7");
        expect(textarea).toHaveAttribute("maxlength", "300");
        expect(textarea).toHaveAttribute("name", "body");
        expect(textarea).toBeRequired();
    });

    it("stops accepting text once the maximum length is reached", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(<TextArea placeholder="Say something" maxLength={4} />);

        // when
        await user.type(screen.getByPlaceholderText("Say something"), "retrospective");

        // then
        expect(screen.getByPlaceholderText("Say something")).toHaveValue("retr");
    });

    it("lets the browser pick the text direction from what is typed", () => {
        // given
        const markup = <TextArea placeholder="Say something" />;

        // when
        renderWithProviders(markup);

        // then
        expect(screen.getByPlaceholderText("Say something")).toHaveAttribute("dir", "auto");
    });

    it("keeps a caller supplied class alongside its own classes", () => {
        // given
        const className = "tall-box";

        // when
        renderWithProviders(<TextArea placeholder="Say something" className={className} />);

        // then
        const textarea = screen.getByPlaceholderText("Say something");
        expect(textarea).toHaveClass(className);
        expect(textarea.className.split(" ").length).toBeGreaterThan(1);
    });
});
