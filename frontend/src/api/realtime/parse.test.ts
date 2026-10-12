import { describe, expect, it } from "vitest";
import { parseServerEvent } from "./parse";

describe("parseServerEvent rejects malformed packets", () => {
    it("rejects json that does not parse", () => {
        // given
        const raw = '{"type":"pong",';

        // when
        const result = parseServerEvent(raw);

        // then
        expect(result).toEqual({ ok: false, reason: "invalid-json" });
    });

    it("rejects an empty string", () => {
        // when
        const result = parseServerEvent("");

        // then
        expect(result).toEqual({ ok: false, reason: "invalid-json" });
    });

    it("rejects a json scalar rather than an object", () => {
        // then
        expect(parseServerEvent("42")).toEqual({ ok: false, reason: "not-an-object" });
        expect(parseServerEvent('"pong"')).toEqual({ ok: false, reason: "not-an-object" });
        expect(parseServerEvent("true")).toEqual({ ok: false, reason: "not-an-object" });
        expect(parseServerEvent("null")).toEqual({ ok: false, reason: "not-an-object" });
    });

    it("rejects a json array", () => {
        // when
        const result = parseServerEvent('[{"type":"pong","data":{}}]');

        // then
        expect(result).toEqual({ ok: false, reason: "not-an-object" });
    });

    it("rejects an object with no type", () => {
        // when
        const result = parseServerEvent('{"data":{"nonce":3}}');

        // then
        expect(result).toEqual({ ok: false, reason: "missing-type" });
    });

    it("rejects an object whose type is not a string", () => {
        // then
        expect(parseServerEvent('{"type":7,"data":{}}')).toEqual({ ok: false, reason: "missing-type" });
        expect(parseServerEvent('{"type":null,"data":{}}')).toEqual({ ok: false, reason: "missing-type" });
    });

    it("rejects an object whose type is not a known event, and names the type it saw", () => {
        // when
        const result = parseServerEvent('{"type":"totally_made_up","data":{}}');

        // then
        expect(result).toEqual({ ok: false, reason: "unknown-type", type: "totally_made_up" });
    });

    it("never throws, whatever arrives on the wire", () => {
        // given
        const hostile = [
            "",
            " ",
            "{",
            "}",
            "[",
            "undefined",
            "NaN",
            "'single quoted'",
            '{"type":',
            '{"type":"pong","data":',
            '{"type":{"nested":true}}',
            "[]{}",
            '{"type":"pong"}{"type":"pong"}',
        ];

        // then
        for (const raw of hostile) {
            expect(() => parseServerEvent(raw)).not.toThrow();
            expect(parseServerEvent(raw).ok).toBe(false);
        }
    });
});

describe("parseServerEvent accepts well formed packets", () => {
    it("returns the event under its declared name", () => {
        // given
        const raw = JSON.stringify({ type: "pong", data: { nonce: 3, server_now: "2026-09-22T10:06:02.512Z" } });

        // when
        const result = parseServerEvent(raw);

        // then
        expect(result).toEqual({
            ok: true,
            event: { type: "pong", data: { nonce: 3, server_now: "2026-09-22T10:06:02.512Z" } },
        });
    });

    it("delivers a null payload rather than rejecting it, because parse judges the envelope only", () => {
        // given
        const raw = JSON.stringify({ type: "pong", data: null });

        // when
        const result = parseServerEvent(raw);

        // then
        expect(result).toEqual({ ok: true, event: { type: "pong", data: null } });
    });

    it("does not apply the event guards, which are the dispatcher's opt-in job", () => {
        // given
        const raw = JSON.stringify({ type: "pong", data: { nonce: "not a number", server_now: 42 } });

        // when
        const result = parseServerEvent(raw);

        // then
        expect(result).toEqual({ ok: true, event: { type: "pong", data: { nonce: "not a number", server_now: 42 } } });
    });

    it("leaves the payload exactly as it arrived, fields it does not know included", () => {
        // given
        const data = { nonce: 1, server_now: "2026-09-22T10:06:02.512Z", extra: "kept" };
        const raw = JSON.stringify({ type: "pong", data });

        // when
        const result = parseServerEvent(raw);

        // then
        expect(result).toEqual({ ok: true, event: { type: "pong", data } });
    });
});

describe("parseServerEvent weighs the declared type against the known event names", () => {
    it("accepts a declared event name and rejects anything else", () => {
        // then
        expect(parseServerEvent('{"type":"pong","data":{}}').ok).toBe(true);
        expect(parseServerEvent('{"type":"ping","data":{}}')).toEqual({
            ok: false,
            reason: "unknown-type",
            type: "ping",
        });
        expect(parseServerEvent('{"type":"PONG","data":{}}')).toEqual({
            ok: false,
            reason: "unknown-type",
            type: "PONG",
        });
        expect(parseServerEvent('{"type":"toString","data":{}}')).toEqual({
            ok: false,
            reason: "unknown-type",
            type: "toString",
        });
        expect(parseServerEvent('{"type":"","data":{}}')).toEqual({ ok: false, reason: "unknown-type", type: "" });
    });

    it.each([
        "error",
        "board_joined",
        "board_join_rejected",
        "board_access_revoked",
        "board_presence",
        "board_events",
        "board_me",
        "team_action_items_changed",
    ])("knows the %s frame a live board sends", type => {
        // then
        expect(parseServerEvent(JSON.stringify({ type, data: {} })).ok).toBe(true);
    });

    it("does not know the commands the client itself sends", () => {
        // then
        expect(parseServerEvent('{"type":"board_join","data":{}}')).toEqual({
            ok: false,
            reason: "unknown-type",
            type: "board_join",
        });
    });
});
