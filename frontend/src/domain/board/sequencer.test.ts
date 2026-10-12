import { describe, expect, it } from "vitest";
import {
    receiveFrame,
    receiveJoined,
    receiveRefusal,
    type SequencedEvent,
    type SequencedFrame,
    type SequencerState,
    settleSequencer,
    startSequencer,
} from "./sequencer";

type TestEvent = SequencedEvent;

function event(version: number, type = "card_upserted"): TestEvent {
    return { type, version };
}

function frame(from: number, to: number, events: TestEvent[] = []): SequencedFrame<TestEvent> {
    return { from, to, events };
}

function live(floor: number): SequencerState<TestEvent> {
    return { phase: "live", floor, buffered: [] };
}

describe("startSequencer", () => {
    it("starts out joining, with nothing buffered and no floor", () => {
        // when
        const state = startSequencer<TestEvent>();

        // then
        expect(state).toEqual({ phase: "joining", floor: null, buffered: [] });
    });
});

describe("the join handshake", () => {
    it("buffers every frame that arrives before the board has answered the join", () => {
        // given
        const early = frame(11, 11, [event(11)]);

        // when
        const step = receiveFrame(startSequencer<TestEvent>(), 10, early);

        // then
        expect(step.apply).toBeNull();
        expect(step.refetch).toBe(false);
        expect(step.state.buffered).toEqual([early]);
        expect(step.state.phase).toBe("joining");
    });

    it("goes live at once when the board held is already at the joined version", () => {
        // when
        const step = receiveJoined(startSequencer<TestEvent>(), 10, 10);

        // then
        expect(step.state.phase).toBe("live");
        expect(step.refetch).toBe(false);
        expect(step.apply).toBeNull();
    });

    it("applies the frames buffered during the join once the board held is at the joined version", () => {
        // given
        const buffered = receiveFrame(startSequencer<TestEvent>(), 10, frame(11, 12, [event(11), event(12)])).state;

        // when
        const step = receiveJoined(buffered, 10, 10);

        // then
        expect(step.apply).toEqual({ events: [event(11), event(12)], to: 12 });
        expect(step.state.phase).toBe("live");
        expect(step.state.buffered).toEqual([]);
    });

    it("asks for the board when the one held is older than the joined version", () => {
        // when
        const step = receiveJoined(startSequencer<TestEvent>(), 7, 10);

        // then
        expect(step.refetch).toBe(true);
        expect(step.apply).toBeNull();
        expect(step.state).toEqual({ phase: "catching-up", floor: 10, buffered: [] });
    });

    it("asks for the board when none is held yet", () => {
        // when
        const step = receiveJoined(startSequencer<TestEvent>(), null, 10);

        // then
        expect(step.refetch).toBe(true);
        expect(step.state.phase).toBe("catching-up");
    });

    it("keeps buffering while the board is being fetched", () => {
        // given
        const catchingUp = receiveJoined(startSequencer<TestEvent>(), 7, 10).state;

        // when
        const step = receiveFrame(catchingUp, 7, frame(11, 11, [event(11)]));

        // then
        expect(step.apply).toBeNull();
        expect(step.refetch).toBe(false);
        expect(step.state.buffered).toHaveLength(1);
    });

    it("applies only the buffered events above the fetched snapshot, then goes live", () => {
        // given
        let state = receiveJoined(startSequencer<TestEvent>(), 7, 10).state;
        state = receiveFrame(state, 7, frame(11, 12, [event(11), event(12)])).state;
        state = receiveFrame(state, 7, frame(13, 13, [event(13)])).state;

        // when
        const step = settleSequencer(state, 12);

        // then
        expect(step.apply).toEqual({ events: [event(13)], to: 13 });
        expect(step.state.phase).toBe("live");
        expect(step.state.buffered).toEqual([]);
    });

    it("drops every buffered frame the fetched snapshot already holds", () => {
        // given
        let state = receiveJoined(startSequencer<TestEvent>(), 7, 10).state;
        state = receiveFrame(state, 7, frame(11, 12, [event(11), event(12)])).state;

        // when
        const step = settleSequencer(state, 14);

        // then
        expect(step.apply).toBeNull();
        expect(step.state).toEqual({ phase: "live", floor: 10, buffered: [] });
    });

    it("asks again when the fetched snapshot is still older than the joined version, because it was read before the join", () => {
        // given
        const state = receiveJoined(startSequencer<TestEvent>(), null, 10).state;

        // when
        const step = settleSequencer(state, 9);

        // then
        expect(step.refetch).toBe(true);
        expect(step.state.phase).toBe("catching-up");
    });

    it("does nothing on a settle before the board has answered, since there is no floor to judge against", () => {
        // given
        const state = receiveFrame(startSequencer<TestEvent>(), 3, frame(4, 4, [event(4)])).state;

        // when
        const step = settleSequencer(state, 3);

        // then
        expect(step).toEqual({ state, apply: null, refetch: false });
    });
});

describe("frames while live", () => {
    it("applies a frame that starts one past the version applied", () => {
        // when
        const step = receiveFrame(live(10), 10, frame(11, 12, [event(11), event(12)]));

        // then
        expect(step.apply).toEqual({ events: [event(11), event(12)], to: 12 });
        expect(step.refetch).toBe(false);
        expect(step.state.phase).toBe("live");
    });

    it("moves the version past a frame whose versions the viewer may not see", () => {
        // when
        const step = receiveFrame(live(10), 10, frame(11, 13, [event(13)]));

        // then
        expect(step.apply).toEqual({ events: [event(13)], to: 13 });
    });

    it("drops a frame it has already applied, such as the echo of a change a refetch brought first", () => {
        // when
        const step = receiveFrame(live(10), 12, frame(11, 12, [event(11), event(12)]));

        // then
        expect(step.apply).toBeNull();
        expect(step.refetch).toBe(false);
        expect(step.state.buffered).toEqual([]);
    });

    it("applies only the events above the version applied from a frame that overlaps it", () => {
        // when
        const step = receiveFrame(live(10), 11, frame(11, 13, [event(11), event(12), event(13)]));

        // then
        expect(step.apply).toEqual({ events: [event(12), event(13)], to: 13 });
    });

    it("buffers a frame after a gap and asks for the board once", () => {
        // given
        const lost = frame(13, 13, [event(13)]);

        // when
        const step = receiveFrame(live(10), 11, lost);

        // then
        expect(step.apply).toBeNull();
        expect(step.refetch).toBe(true);
        expect(step.state).toEqual({ phase: "catching-up", floor: 10, buffered: [lost] });
    });

    it("does not ask again for frames that arrive while it is catching up after a gap", () => {
        // given
        const gapped = receiveFrame(live(10), 11, frame(13, 13, [event(13)])).state;

        // when
        const step = receiveFrame(gapped, 11, frame(14, 14, [event(14)]));

        // then
        expect(step.refetch).toBe(false);
        expect(step.state.buffered).toHaveLength(2);
    });

    it("applies the frames after the gap once the refetched board covers it", () => {
        // given
        let state = receiveFrame(live(10), 11, frame(13, 13, [event(13)])).state;
        state = receiveFrame(state, 11, frame(14, 14, [event(14)])).state;

        // when
        const step = settleSequencer(state, 12);

        // then
        expect(step.apply).toEqual({ events: [event(13), event(14)], to: 14 });
        expect(step.state.phase).toBe("live");
    });

    it("asks again when the refetched board still leaves a gap", () => {
        // given
        const state = receiveFrame(live(10), 11, frame(14, 14, [event(14)])).state;

        // when
        const step = settleSequencer(state, 12);

        // then
        expect(step.refetch).toBe(true);
        expect(step.state.buffered).toHaveLength(1);
    });

    it("applies buffered frames in version order, whatever order they were buffered in", () => {
        // given
        let state: SequencerState<TestEvent> = { phase: "catching-up", floor: 10, buffered: [] };
        state = receiveFrame(state, 10, frame(12, 12, [event(12)])).state;
        state = receiveFrame(state, 10, frame(11, 11, [event(11)])).state;

        // when
        const step = settleSequencer(state, 10);

        // then
        expect(step.apply).toEqual({ events: [event(11), event(12)], to: 12 });
    });
});

describe("resync", () => {
    it("applies the events before a resync, stops there and asks for the board", () => {
        // given
        const withResync = frame(11, 13, [event(11), event(12, "resync"), event(13)]);

        // when
        const step = receiveFrame(live(10), 10, withResync);

        // then
        expect(step.apply).toEqual({ events: [event(11)], to: 11 });
        expect(step.refetch).toBe(true);
        expect(step.state.phase).toBe("catching-up");
        expect(step.state.buffered).toEqual([withResync]);
    });

    it("asks for the board without applying anything when the resync comes first", () => {
        // when
        const step = receiveFrame(live(10), 10, frame(11, 11, [event(11, "resync")]));

        // then
        expect(step.apply).toBeNull();
        expect(step.refetch).toBe(true);
    });

    it("finishes the frame after a refetch that covers the resync", () => {
        // given
        const state = receiveFrame(live(10), 10, frame(11, 13, [event(11), event(12, "resync"), event(13)])).state;

        // when
        const step = settleSequencer(state, 12);

        // then
        expect(step.apply).toEqual({ events: [event(13)], to: 13 });
        expect(step.state.phase).toBe("live");
    });
});

describe("refusal", () => {
    it("forgets everything buffered and stops listening", () => {
        // given
        const refused = receiveRefusal<TestEvent>();

        // when
        const joined = receiveJoined(refused, 10, 10);
        const framed = receiveFrame(refused, 10, frame(11, 11, [event(11)]));
        const settled = settleSequencer(refused, 10);

        // then
        expect(refused).toEqual({ phase: "refused", floor: null, buffered: [] });
        expect(joined.state).toBe(refused);
        expect(framed.state).toBe(refused);
        expect(settled.state).toBe(refused);
        expect(framed.apply).toBeNull();
    });
});
