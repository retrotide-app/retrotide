import { fireEvent, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { BOARD_LAYOUT_ID, BOARD_SEARCH_ID } from "../../domain/board/shortcuts";
import { useBoardShortcuts } from "./useBoardShortcuts";

function plant<T extends HTMLElement>(element: T): T {
    element.setAttribute("data-planted", "");
    document.body.append(element);

    return element;
}

function plantSearch(): HTMLInputElement {
    const search = document.createElement("input");
    search.id = BOARD_SEARCH_ID;

    return plant(search);
}

function plantLayout(): { columns: HTMLButtonElement; list: HTMLButtonElement } {
    const group = document.createElement("div");
    group.id = BOARD_LAYOUT_ID;
    const columns = document.createElement("button");
    columns.setAttribute("aria-pressed", "true");
    const list = document.createElement("button");
    list.setAttribute("aria-pressed", "false");
    group.append(columns, list);
    plant(group);

    return { columns, list };
}

afterEach(() => {
    for (const planted of document.querySelectorAll("[data-planted]")) {
        planted.remove();
    }
});

describe("useBoardShortcuts", () => {
    it("opens the shortcut list on ?", () => {
        // given
        const onHelp = vi.fn();
        renderHook(() => useBoardShortcuts({ active: true, onHelp }));

        // when
        const handled = !fireEvent.keyDown(window, { key: "?" });

        // then
        expect(onHelp).toHaveBeenCalledOnce();
        expect(handled).toBe(true);
    });

    it("focuses the card search on /", () => {
        // given
        const search = plantSearch();
        renderHook(() => useBoardShortcuts({ active: true, onHelp: vi.fn() }));

        // when
        const handled = !fireEvent.keyDown(document.body, { key: "/" });

        // then
        expect(search).toHaveFocus();
        expect(handled).toBe(true);
    });

    it("switches to the layout that is not shown on l", () => {
        // given
        const { columns, list } = plantLayout();
        const chooseColumns = vi.fn();
        const chooseList = vi.fn();
        columns.addEventListener("click", chooseColumns);
        list.addEventListener("click", chooseList);
        renderHook(() => useBoardShortcuts({ active: true, onHelp: vi.fn() }));

        // when
        fireEvent.keyDown(document.body, { key: "l" });

        // then
        expect(chooseList).toHaveBeenCalledOnce();
        expect(chooseColumns).not.toHaveBeenCalled();
        expect(list).toHaveFocus();
    });

    it("opens the first add-card form on n", () => {
        // given
        const first = plant(document.createElement("button"));
        first.setAttribute("data-new-card", "");
        const second = plant(document.createElement("button"));
        second.setAttribute("data-new-card", "");
        const openFirst = vi.fn();
        const openSecond = vi.fn();
        first.addEventListener("click", openFirst);
        second.addEventListener("click", openSecond);
        renderHook(() => useBoardShortcuts({ active: true, onHelp: vi.fn() }));

        // when
        const handled = !fireEvent.keyDown(document.body, { key: "n" });

        // then
        expect(openFirst).toHaveBeenCalledOnce();
        expect(openSecond).not.toHaveBeenCalled();
        expect(handled).toBe(true);
    });

    it("focuses the text of an add-card form that is already open on n", () => {
        // given
        const form = plant(document.createElement("form"));
        form.setAttribute("data-new-card", "");
        const text = document.createElement("textarea");
        form.append(text);
        renderHook(() => useBoardShortcuts({ active: true, onHelp: vi.fn() }));

        // when
        fireEvent.keyDown(document.body, { key: "n" });

        // then
        expect(text).toHaveFocus();
    });

    it("lets the key through when there is nothing to act on", () => {
        // given
        renderHook(() => useBoardShortcuts({ active: true, onHelp: vi.fn() }));

        // when
        const passedThrough = fireEvent.keyDown(document.body, { key: "n" });

        // then
        expect(passedThrough).toBe(true);
    });

    it.each([
        ["an input", "input"],
        ["a text area", "textarea"],
        ["a select", "select"],
    ])("ignores keys typed in %s", (_, tag) => {
        // given
        const onHelp = vi.fn();
        const field = plant(document.createElement(tag));
        renderHook(() => useBoardShortcuts({ active: true, onHelp }));

        // when
        fireEvent.keyDown(field, { key: "?" });

        // then
        expect(onHelp).not.toHaveBeenCalled();
    });

    it("ignores keys typed in an editable element or inside a dialog", () => {
        // given
        const onHelp = vi.fn();
        const editable = plant(document.createElement("div"));
        editable.setAttribute("contenteditable", "true");
        const dialog = plant(document.createElement("div"));
        dialog.setAttribute("role", "dialog");
        const button = document.createElement("button");
        dialog.append(button);
        renderHook(() => useBoardShortcuts({ active: true, onHelp }));

        // when
        fireEvent.keyDown(editable, { key: "?" });
        fireEvent.keyDown(button, { key: "?" });

        // then
        expect(onHelp).not.toHaveBeenCalled();
    });

    it("ignores keys pressed with a modifier held", () => {
        // given
        const search = plantSearch();
        renderHook(() => useBoardShortcuts({ active: true, onHelp: vi.fn() }));

        // when
        fireEvent.keyDown(document.body, { key: "/", ctrlKey: true });
        fireEvent.keyDown(document.body, { key: "/", metaKey: true });
        fireEvent.keyDown(document.body, { key: "/", altKey: true });

        // then
        expect(search).not.toHaveFocus();
    });

    it("stays quiet while inactive, as while a presentation owns the keyboard", () => {
        // given
        const onHelp = vi.fn();
        renderHook(() => useBoardShortcuts({ active: false, onHelp }));

        // when
        fireEvent.keyDown(window, { key: "?" });

        // then
        expect(onHelp).not.toHaveBeenCalled();
    });

    it("stops listening once unmounted", () => {
        // given
        const onHelp = vi.fn();
        const { unmount } = renderHook(() => useBoardShortcuts({ active: true, onHelp }));

        // when
        unmount();
        fireEvent.keyDown(window, { key: "?" });

        // then
        expect(onHelp).not.toHaveBeenCalled();
    });
});
