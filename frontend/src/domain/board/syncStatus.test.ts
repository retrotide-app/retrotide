import { describe, expect, it } from "vitest";
import type { BoardRefusalReason } from "../../types/api";
import { refusalNotice, SYNC_CONNECTING, SYNC_LIVE } from "./syncStatus";

const NEEDS_A_NOTICE: BoardRefusalReason[] = [
    "not_participant",
    "password_required",
    "account_required",
    "link_disabled",
    "removed",
];

describe("refusalNotice", () => {
    it("has nothing to say while the board is live or connecting", () => {
        // then
        expect(refusalNotice(SYNC_LIVE)).toBeNull();
        expect(refusalNotice(SYNC_CONNECTING)).toBeNull();
    });

    it.each(NEEDS_A_NOTICE)("explains the %s refusal in words", reason => {
        // when
        const notice = refusalNotice({ state: "refused", reason, revoked: true });

        // then
        expect(notice).not.toBeNull();
        expect(notice).not.toContain("_");
    });

    it("tells a removed participant who removed them", () => {
        // then
        expect(refusalNotice({ state: "refused", reason: "removed", revoked: true })).toContain(
            "facilitator removed you",
        );
    });

    it("says nothing for an archived board, whose page already says it is archived", () => {
        // then
        expect(refusalNotice({ state: "refused", reason: "archived", revoked: false })).toBeNull();
    });
});
