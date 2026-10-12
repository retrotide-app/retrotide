import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { FormEvent } from "react";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Modal } from "./Modal";

const closeLabel = "Close";

describe("Modal", () => {
    it("renders nothing at all while it is closed", () => {
        // given
        const isOpen = false;

        // when
        const { container } = renderWithProviders(
            <Modal isOpen={isOpen} onClose={vi.fn()} title="Edit details">
                <p>hidden body</p>
            </Modal>,
        );

        // then
        expect(container).toBeEmptyDOMElement();
        expect(screen.queryByText("Edit details")).not.toBeInTheDocument();
    });

    it("shows its title and its children once it is open", () => {
        // given
        const isOpen = true;

        // when
        renderWithProviders(
            <Modal isOpen={isOpen} onClose={vi.fn()} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );

        // then
        expect(screen.getByRole("heading", { name: "Edit details" })).toBeInTheDocument();
        expect(screen.getByText("dialog content")).toBeInTheDocument();
    });

    it("renders into the document body rather than inside the element that holds it", () => {
        // given
        const isOpen = true;

        // when
        renderWithProviders(
            <section aria-label="Host">
                <Modal isOpen={isOpen} onClose={vi.fn()} title="Edit details">
                    <p>dialog content</p>
                </Modal>
            </section>,
        );

        // then
        const dialog = screen.getByRole("dialog", { name: "Edit details" });
        expect(screen.getByRole("region", { name: "Host" })).not.toContainElement(dialog);
        expect(document.body).toContainElement(dialog);
    });

    it("names its close control Close for assistive technology", () => {
        // given
        const isOpen = true;

        // when
        renderWithProviders(
            <Modal isOpen={isOpen} onClose={vi.fn()} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );

        // then
        expect(screen.getByRole("button", { name: closeLabel })).toHaveAttribute("type", "button");
    });

    it("closes when the close control is pressed", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <Modal isOpen onClose={onClose} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );

        // when
        await user.click(screen.getByRole("button", { name: closeLabel }));

        // then
        expect(onClose).toHaveBeenCalledOnce();
    });

    it("closes when the backdrop behind it is clicked", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <Modal isOpen onClose={onClose} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );

        // when
        await user.click(screen.getByRole("dialog").parentElement as HTMLElement);

        // then
        expect(onClose).toHaveBeenCalledOnce();
    });

    it("stays open when the backdrop is clicked while backdrop clicks are switched off", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <Modal isOpen onClose={onClose} title="New password" closeOnOverlayClick={false}>
                <p>dialog content</p>
            </Modal>,
        );

        // when
        await user.click(screen.getByRole("dialog").parentElement as HTMLElement);

        // then
        expect(onClose).not.toHaveBeenCalled();
        expect(screen.getByRole("dialog", { name: "New password" })).toBeInTheDocument();
    });

    it("still closes from its close control while backdrop clicks are switched off", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <Modal isOpen onClose={onClose} title="New password" closeOnOverlayClick={false}>
                <p>dialog content</p>
            </Modal>,
        );

        // when
        await user.click(screen.getByRole("button", { name: closeLabel }));

        // then
        expect(onClose).toHaveBeenCalledOnce();
    });

    it("stays open when the panel itself is clicked", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <Modal isOpen onClose={onClose} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );

        // when
        await user.click(screen.getByText("dialog content"));

        // then
        expect(onClose).not.toHaveBeenCalled();
    });

    it("hides its whole subtree again when it is closed after being open", () => {
        // given
        const onClose = vi.fn();
        const { rerender } = renderWithProviders(
            <Modal isOpen onClose={onClose} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );

        // when
        rerender(
            <Modal isOpen={false} onClose={onClose} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );

        // then
        expect(screen.queryByText("dialog content")).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: closeLabel })).not.toBeInTheDocument();
    });

    it("puts the close control in the tab order ahead of its own content", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(
            <Modal isOpen onClose={vi.fn()} title="Edit details">
                <button type="button">Confirm</button>
            </Modal>,
        );

        // when
        await user.tab();

        // then
        expect(screen.getByRole("button", { name: closeLabel })).toHaveFocus();
    });

    it("announces itself as a modal dialog named after its title", () => {
        // given
        const title = "Edit details";

        // when
        renderWithProviders(
            <Modal isOpen onClose={vi.fn()} title={title}>
                <p>dialog content</p>
            </Modal>,
        );

        // then
        const dialog = screen.getByRole("dialog", { name: title });
        expect(dialog).toHaveAttribute("aria-modal", "true");
        expect(dialog).toContainElement(screen.getByRole("heading", { name: title }));
    });

    it("moves focus into the dialog when it opens", () => {
        // given
        const onClose = vi.fn();
        const { rerender } = renderWithProviders(
            <>
                <button type="button">Open the dialog</button>
                <Modal isOpen={false} onClose={onClose} title="Edit details">
                    <p>dialog content</p>
                </Modal>
            </>,
        );
        screen.getByRole("button", { name: "Open the dialog" }).focus();

        // when
        rerender(
            <>
                <button type="button">Open the dialog</button>
                <Modal isOpen onClose={onClose} title="Edit details">
                    <p>dialog content</p>
                </Modal>
            </>,
        );

        // then
        expect(screen.getByRole("dialog")).toHaveFocus();
    });

    it("gives focus back to whatever held it once it closes", () => {
        // given
        const onClose = vi.fn();
        const { rerender } = renderWithProviders(
            <>
                <button type="button">Open the dialog</button>
                <Modal isOpen={false} onClose={onClose} title="Edit details">
                    <p>dialog content</p>
                </Modal>
            </>,
        );
        const trigger = screen.getByRole("button", { name: "Open the dialog" });
        trigger.focus();
        rerender(
            <>
                <button type="button">Open the dialog</button>
                <Modal isOpen onClose={onClose} title="Edit details">
                    <p>dialog content</p>
                </Modal>
            </>,
        );
        screen.getByRole("button", { name: closeLabel }).focus();

        // when
        rerender(
            <>
                <button type="button">Open the dialog</button>
                <Modal isOpen={false} onClose={onClose} title="Edit details">
                    <p>dialog content</p>
                </Modal>
            </>,
        );

        // then
        expect(trigger).toHaveFocus();
    });

    it("closes when the escape key is pressed", async () => {
        // given
        const onClose = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(
            <Modal isOpen onClose={onClose} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );

        // when
        await user.keyboard("{Escape}");

        // then
        expect(onClose).toHaveBeenCalledOnce();
    });

    it("stops listening for escape once it has closed", () => {
        // given
        const onClose = vi.fn();
        const { rerender } = renderWithProviders(
            <Modal isOpen onClose={onClose} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );

        // when
        rerender(
            <Modal isOpen={false} onClose={onClose} title="Edit details">
                <p>dialog content</p>
            </Modal>,
        );
        fireEvent.keyDown(document, { key: "Escape" });

        // then
        expect(onClose).not.toHaveBeenCalled();
    });

    it("does not submit a form it is rendered inside when the close control is pressed", async () => {
        // given
        const onSubmit = vi.fn((event: FormEvent) => {
            event.preventDefault();
        });
        const user = userEvent.setup();
        renderWithProviders(
            <form onSubmit={onSubmit}>
                <Modal isOpen onClose={vi.fn()} title="Edit details">
                    <p>dialog content</p>
                </Modal>
            </form>,
        );

        // when
        await user.click(screen.getByRole("button", { name: closeLabel }));

        // then
        expect(onSubmit).not.toHaveBeenCalled();
    });
});
