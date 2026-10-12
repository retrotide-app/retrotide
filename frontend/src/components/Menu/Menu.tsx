import { type KeyboardEvent as ReactKeyboardEvent, useEffect, useId, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { popupPosition } from "../../utils/popupPosition";
import { Button } from "../Button/Button";
import type { ButtonSize } from "../Button/buttonClassName";
import styles from "./Menu.module.css";

export interface MenuAction {
    label: string;
    onSelect: () => void;
    tone?: "default" | "danger";
}

export interface MenuLink {
    label: string;
    href: string;
    download?: boolean;
    newTab?: boolean;
}

export type MenuItem = MenuAction | MenuLink;

interface MenuProps {
    label: string;
    items: readonly MenuItem[];
    iconOnly?: boolean;
    size?: ButtonSize;
}

function menuItemsOf(menu: HTMLElement | null): HTMLElement[] {
    if (menu === null) {
        return [];
    }

    return Array.from(menu.querySelectorAll<HTMLElement>('[role="menuitem"]'));
}

function nextIndex(key: string, current: number, count: number): number | null {
    if (key === "ArrowDown") {
        return (current + 1) % count;
    }

    if (key === "ArrowUp") {
        return (current - 1 + count) % count;
    }

    if (key === "Home") {
        return 0;
    }

    if (key === "End") {
        return count - 1;
    }

    return null;
}

export function Menu({ label, items, iconOnly = false, size = "medium" }: MenuProps) {
    const menuId = useId();
    const rootRef = useRef<HTMLDivElement>(null);
    const menuRef = useRef<HTMLDivElement>(null);
    const triggerRef = useRef<HTMLButtonElement>(null);
    const [open, setOpen] = useState(false);

    useLayoutEffect(() => {
        if (!open) {
            return;
        }

        function place() {
            const menu = menuRef.current;
            const trigger = triggerRef.current;

            if (menu === null || trigger === null) {
                return;
            }

            const viewport = { width: document.documentElement.clientWidth, height: window.innerHeight };
            const rtl = getComputedStyle(menu).direction === "rtl";
            const position = popupPosition(trigger.getBoundingClientRect(), menu.offsetHeight, viewport, rtl);

            menu.style.top = `${position.top}px`;
            menu.style.insetInlineEnd = `${position.insetInlineEnd}px`;
        }

        place();
        window.addEventListener("resize", place);
        window.addEventListener("scroll", place, true);

        return () => {
            window.removeEventListener("resize", place);
            window.removeEventListener("scroll", place, true);
        };
    }, [open]);

    useEffect(() => {
        if (!open) {
            return;
        }

        menuItemsOf(menuRef.current)[0]?.focus({ preventScroll: true });

        function handlePointerDown(event: PointerEvent) {
            const target = event.target;
            const inside =
                target instanceof Node &&
                (rootRef.current?.contains(target) === true || menuRef.current?.contains(target) === true);

            if (inside) {
                return;
            }

            setOpen(false);
        }

        document.addEventListener("pointerdown", handlePointerDown);

        return () => {
            document.removeEventListener("pointerdown", handlePointerDown);
        };
    }, [open]);

    function close() {
        setOpen(false);
        triggerRef.current?.focus();
    }

    function choose(item: MenuAction) {
        close();
        item.onSelect();
    }

    function handleTriggerKeyDown(event: ReactKeyboardEvent<HTMLButtonElement>) {
        if (event.key !== "ArrowDown" || open) {
            return;
        }

        event.preventDefault();
        setOpen(true);
    }

    function handleMenuKeyDown(event: ReactKeyboardEvent<HTMLDivElement>) {
        if (event.key === "Escape") {
            event.preventDefault();
            close();
            return;
        }

        if (event.key === "Tab") {
            setOpen(false);
            return;
        }

        const options = menuItemsOf(menuRef.current);
        const current = options.findIndex(option => option === document.activeElement);
        const target = nextIndex(event.key, current, options.length);

        if (target === null) {
            return;
        }

        event.preventDefault();
        options[target]?.focus();
    }

    if (items.length === 0) {
        return null;
    }

    return (
        <div ref={rootRef} className={styles.root}>
            <Button
                ref={triggerRef}
                type="button"
                size={size}
                variant={iconOnly ? "ghost" : "secondary"}
                aria-haspopup="menu"
                aria-expanded={open}
                aria-controls={open ? menuId : undefined}
                aria-label={iconOnly ? label : undefined}
                onClick={() => setOpen(current => !current)}
                onKeyDown={handleTriggerKeyDown}
            >
                {iconOnly ? (
                    <span className={styles.dots} aria-hidden="true">
                        {"⋯"}
                    </span>
                ) : (
                    <>
                        {label}
                        <span className={styles.caret} aria-hidden="true">
                            {"▾"}
                        </span>
                    </>
                )}
            </Button>

            {open &&
                createPortal(
                    <div
                        ref={menuRef}
                        id={menuId}
                        role="menu"
                        aria-label={label}
                        className={styles.menu}
                        onKeyDown={handleMenuKeyDown}
                    >
                        {items.map(item =>
                            "href" in item ? (
                                <a
                                    key={item.label}
                                    href={item.href}
                                    download={item.download === true ? "" : undefined}
                                    target={item.newTab === true ? "_blank" : undefined}
                                    rel={item.newTab === true ? "noopener" : undefined}
                                    role="menuitem"
                                    tabIndex={-1}
                                    className={styles.item}
                                    onClick={close}
                                >
                                    {item.label}
                                </a>
                            ) : (
                                <button
                                    key={item.label}
                                    type="button"
                                    role="menuitem"
                                    tabIndex={-1}
                                    className={item.tone === "danger" ? `${styles.item} ${styles.danger}` : styles.item}
                                    onClick={() => choose(item)}
                                >
                                    {item.label}
                                </button>
                            ),
                        )}
                    </div>,
                    document.body,
                )}
        </div>
    );
}
