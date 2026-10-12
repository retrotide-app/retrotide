import { act, render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { FakeWebSocket } from "../../../test-utils/ws";
import type { Viewer } from "../../../types/app";
import { subscribe } from "../bus";
import { RealtimeSync } from "./RealtimeSync";

const signedIn: Viewer = { kind: "user", id: "u1" };
const guest: Viewer = { kind: "guest", id: "g1" };

function lastSocket(): FakeWebSocket {
    const socket = FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
    if (!socket) {
        throw new Error("no websocket was opened");
    }

    return socket;
}

function liveSockets(): FakeWebSocket[] {
    return FakeWebSocket.instances.filter(instance => instance.readyState === FakeWebSocket.OPEN);
}

beforeEach(() => {
    FakeWebSocket.instances = [];
    vi.stubGlobal("WebSocket", FakeWebSocket);
});

describe("RealtimeSync", () => {
    it("opens no socket while there is no viewer", () => {
        // given
        const nobody = null;

        // when
        render(<RealtimeSync viewer={nobody} />);

        // then
        expect(FakeWebSocket.instances).toHaveLength(0);
    });

    it("opens exactly one socket for a signed in user", () => {
        // when
        render(<RealtimeSync viewer={signedIn} />);

        // then
        expect(FakeWebSocket.instances).toHaveLength(1);
        expect(lastSocket().url).toBe("ws://localhost:3000/api/v1/ws");
    });

    it("opens one for a guest too, because guests see the board live", () => {
        // when
        render(<RealtimeSync viewer={guest} />);

        // then
        expect(FakeWebSocket.instances).toHaveLength(1);
        expect(lastSocket().readyState).toBe(FakeWebSocket.OPEN);
    });

    it("closes the socket when it unmounts", () => {
        // given
        const { unmount } = render(<RealtimeSync viewer={signedIn} />);
        const socket = lastSocket();

        // when
        unmount();

        // then
        expect(socket.closeCount).toBe(1);
        expect(liveSockets()).toHaveLength(0);
    });

    it("closes the old socket and opens a new one when the viewer changes", () => {
        // given
        const { rerender } = render(<RealtimeSync viewer={guest} />);
        const before = lastSocket();

        // when
        rerender(<RealtimeSync viewer={signedIn} />);

        // then
        expect(FakeWebSocket.instances).toHaveLength(2);
        expect(before.closeCount).toBe(1);
        expect(liveSockets()).toEqual([lastSocket()]);
    });

    it("treats a user and a guest with the same id as different sessions", () => {
        // given
        const { rerender } = render(<RealtimeSync viewer={{ kind: "guest", id: "same" }} />);
        const before = lastSocket();

        // when
        rerender(<RealtimeSync viewer={{ kind: "user", id: "same" }} />);

        // then
        expect(before.closeCount).toBe(1);
        expect(liveSockets()).toEqual([lastSocket()]);
    });

    it("keeps the socket when a re-render brings the same viewer in a new object", () => {
        // given
        const { rerender } = render(<RealtimeSync viewer={{ kind: "user", id: "u1" }} />);
        const opened = lastSocket();

        // when
        rerender(<RealtimeSync viewer={{ kind: "user", id: "u1" }} />);

        // then
        expect(FakeWebSocket.instances).toHaveLength(1);
        expect(opened.closeCount).toBe(0);
    });

    it("closes the socket when the viewer goes away", () => {
        // given
        const { rerender } = render(<RealtimeSync viewer={signedIn} />);
        const opened = lastSocket();

        // when
        rerender(<RealtimeSync viewer={null} />);

        // then
        expect(opened.closeCount).toBe(1);
        expect(liveSockets()).toHaveLength(0);
    });

    it("installs the pipeline, so a frame off the socket reaches the bus", () => {
        // given
        const handler = vi.fn();
        const unsubscribe = subscribe("pong", handler);
        render(<RealtimeSync viewer={signedIn} />);

        // when
        act(() => {
            lastSocket().onopen?.();
            lastSocket().onmessage?.({ data: JSON.stringify({ type: "pong", data: { nonce: 1 } }) });
        });
        unsubscribe();

        // then
        expect(handler).toHaveBeenCalledWith({ type: "pong", data: { nonce: 1 } });
    });

    it("renders nothing", () => {
        // when
        const { container } = render(<RealtimeSync viewer={signedIn} />);

        // then
        expect(container).toBeEmptyDOMElement();
    });
});
