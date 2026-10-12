import { beforeEach, describe, expect, it, vi } from "vitest";
import { REALTIME_COMMANDS, sendRealtime, type RealtimeCommand, type RealtimeCommandName } from "./outbound";

const { sendRaw } = vi.hoisted(() => ({ sendRaw: vi.fn() }));

vi.mock("./socket", () => ({ sendRaw }));

function lastFrame(): string {
    const call = sendRaw.mock.calls[sendRaw.mock.calls.length - 1];
    if (!call) {
        throw new Error("no frame was sent");
    }

    return call[0] as string;
}

const commands: { name: RealtimeCommandName; command: RealtimeCommand; wire: string }[] = [
    {
        name: "ping",
        command: { type: "ping", data: { nonce: 4 } },
        wire: '{"type":"ping","data":{"nonce":4}}',
    },
    {
        name: "board_join",
        command: { type: "board_join", data: { board_id: "8f0c" } },
        wire: '{"type":"board_join","data":{"board_id":"8f0c"}}',
    },
    {
        name: "board_leave",
        command: { type: "board_leave", data: { board_id: "8f0c" } },
        wire: '{"type":"board_leave","data":{"board_id":"8f0c"}}',
    },
];

beforeEach(() => {
    sendRaw.mockClear();
});

describe("realtime outbound wire format", () => {
    for (const entry of commands) {
        it(`serialises ${entry.name} with the field names the server parses`, () => {
            // when
            sendRealtime(entry.command);

            // then
            expect(sendRaw).toHaveBeenCalledExactlyOnceWith(entry.wire);
        });
    }

    it("sends a keepalive ping with no nonce, which the server answers with a pong that carries none", () => {
        // when
        sendRealtime({ type: "ping", data: {} });

        // then
        expect(lastFrame()).toBe('{"type":"ping","data":{}}');
    });

    it("carries the measured round trip beside the nonce when the caller has one", () => {
        // when
        sendRealtime({ type: "ping", data: { nonce: 5, rtt: 42 } });

        // then
        expect(lastFrame()).toBe('{"type":"ping","data":{"nonce":5,"rtt":42}}');
    });

    it("sends one frame per call and nothing on its own", () => {
        // given
        expect(sendRaw).not.toHaveBeenCalled();

        // when
        sendRealtime({ type: "ping", data: { nonce: 1 } });
        sendRealtime({ type: "ping", data: { nonce: 2 } });

        // then
        expect(sendRaw).toHaveBeenCalledTimes(2);
    });

    it("hands the transport a string, never an object", () => {
        // when
        sendRealtime({ type: "ping", data: { nonce: 9 } });

        // then
        expect(typeof lastFrame()).toBe("string");
        expect(JSON.parse(lastFrame())).toEqual({ type: "ping", data: { nonce: 9 } });
    });
});

describe("realtime command name table", () => {
    it("names every command the union declares and no others", () => {
        // given
        const declared = commands.map(entry => entry.name);

        // when
        const table = Object.values(REALTIME_COMMANDS);

        // then
        expect([...table].sort()).toEqual([...declared].sort());
    });

    it("has the liveness ping and the board join and leave, and nothing that writes to a board", () => {
        // when
        const table = Object.values(REALTIME_COMMANDS);

        // then
        expect(table).toEqual(["ping", "board_join", "board_leave"]);
    });
});
