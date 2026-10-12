export interface AnchorBox {
    top: number;
    bottom: number;
    left: number;
    right: number;
}

export interface Viewport {
    width: number;
    height: number;
}

export interface PopupPosition {
    top: number;
    insetInlineEnd: number;
}

export const POPUP_GAP = 6;

export function popupPosition(anchor: AnchorBox, popupHeight: number, viewport: Viewport, rtl: boolean): PopupPosition {
    const insetInlineEnd = rtl ? anchor.left : viewport.width - anchor.right;
    const below = anchor.bottom + POPUP_GAP;
    const above = anchor.top - POPUP_GAP - popupHeight;
    const fitsBelow = below + popupHeight <= viewport.height;

    if (fitsBelow || above < 0) {
        return { top: below, insetInlineEnd };
    }

    return { top: above, insetInlineEnd };
}
