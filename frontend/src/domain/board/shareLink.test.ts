import { describe, expect, it } from "vitest";
import { makeApiError, makeBoardSnapshot, makeBoardView, makeParticipantViewer } from "../../test-utils/fixtures";
import {
    canShareBoard,
    GUEST_NAME_MAX_LENGTH,
    joinedBoardPath,
    joinPasswordRefusal,
    joinProblems,
    joinRefusal,
    joinRequest,
    LINK_ACCESS_OPTIONS,
    needsAccount,
    rejoinSlugFrom,
    rejoinState,
    sharedBoardRefusal,
    shareLinkUrl,
} from "./shareLink";

const named = { name: "Robin", anonymous: false, password: "" };

describe("joinedBoardPath", () => {
    const organisations = [{ id: "o1", slug: "harbour" }];

    it("opens the board inside the member's organisation", () => {
        // when
        const path = joinedBoardPath({ board_id: "b1", org_id: "o1" }, organisations);

        // then
        expect(path).toBe("/o/harbour/boards/b1");
    });

    it("opens the shared board page for a guest or an outside account", () => {
        // when
        const guest = joinedBoardPath({ board_id: "b1" }, organisations);
        const unknown = joinedBoardPath({ board_id: "b1", org_id: "o9" }, organisations);

        // then
        expect(guest).toBe("/boards/b1");
        expect(unknown).toBe("/boards/b1");
    });
});

describe("shareLinkUrl", () => {
    it("joins the page's origin to the share link path", () => {
        // when
        const url = shareLinkUrl("https://retrotide.app", "k7Q2");

        // then
        expect(url).toBe("https://retrotide.app/b/k7Q2");
    });
});

describe("LINK_ACCESS_OPTIONS", () => {
    it("offers every level of link access once", () => {
        // when
        const values = LINK_ACCESS_OPTIONS.map(option => option.value);

        // then
        expect(values).toEqual(["guests", "accounts", "off"]);
    });
});

describe("GUEST_NAME_MAX_LENGTH", () => {
    it("matches the server's 40 character clamp", () => {
        // then
        expect(GUEST_NAME_MAX_LENGTH).toBe(40);
    });
});

describe("canShareBoard", () => {
    it("lets a viewer who manages the board share it", () => {
        // when
        const shares = canShareBoard(makeBoardSnapshot());

        // then
        expect(shares).toBe(true);
    });

    it("never lets a participant share the board, because the link is a capability", () => {
        // given
        const snapshot = makeBoardSnapshot({ viewer: makeParticipantViewer() });

        // when
        const shares = canShareBoard(snapshot);

        // then
        expect(shares).toBe(false);
    });

    it("does not offer sharing on an archived board, which refuses every command", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: makeBoardView({ archived_at: "2026-10-02T09:00:00Z" }) });

        // when
        const shares = canShareBoard(snapshot);

        // then
        expect(shares).toBe(false);
    });
});

describe("joinProblems", () => {
    it("asks a guest for a name unless they chose Anonymous", () => {
        // when
        const problems = joinProblems({ ...named, name: "  " }, true, false);

        // then
        expect(problems).toEqual({ name: "Enter your name, or choose to join as Anonymous." });
    });

    it("needs no name from a guest joining as Anonymous", () => {
        // when
        const problems = joinProblems({ name: "", anonymous: true, password: "" }, true, false);

        // then
        expect(problems).toEqual({});
    });

    it("needs no name from someone signed in, because the account names them", () => {
        // when
        const problems = joinProblems({ ...named, name: "" }, false, false);

        // then
        expect(problems).toEqual({});
    });

    it("asks for the password when the board has one", () => {
        // when
        const problems = joinProblems(named, true, true);

        // then
        expect(problems).toEqual({ password: "Enter the board's password." });
    });
});

describe("joinRequest", () => {
    it("sends a guest's trimmed name", () => {
        // when
        const request = joinRequest({ ...named, name: "  Robin  " }, true, false);

        // then
        expect(request).toEqual({ display_name: "Robin" });
    });

    it("sends no name for a guest joining as Anonymous, even one they typed first", () => {
        // when
        const request = joinRequest({ ...named, anonymous: true }, true, false);

        // then
        expect(request).toEqual({});
    });

    it("never sends a name for someone signed in", () => {
        // when
        const request = joinRequest(named, false, false);

        // then
        expect(request).toEqual({});
    });

    it("sends the password only when the board asks for one", () => {
        // given
        const draft = { ...named, password: "tide" };

        // when
        const withPassword = joinRequest(draft, true, true);
        const without = joinRequest(draft, true, false);

        // then
        expect(withPassword).toEqual({ display_name: "Robin", password: "tide" });
        expect(without).toEqual({ display_name: "Robin" });
    });
});

describe("joinRefusal", () => {
    it.each([
        ["account_required", 403, "This board is only for people with an account. Sign in to join it."],
        ["guests_not_allowed", 403, "This organisation does not let guests join its boards. Sign in to join it."],
        ["link_disabled", 403, "The link to this board is switched off. Ask whoever shared it to switch it back on."],
        ["removed", 403, "A facilitator removed you from this board, so you cannot join it again."],
        ["archived", 409, "This board is archived, so nobody can join it."],
        ["board_full", 429, "This board already holds as many people as it can."],
        ["password_attempts", 429, "Too many wrong passwords for this board. Wait a few minutes, then try again."],
        ["board_busy", 503, "The board is busy. Try again in a moment."],
    ])("explains the %s refusal", (code, status, expected) => {
        // given
        const error = makeApiError({ status, message: "refused", code });

        // when
        const message = joinRefusal(error);

        // then
        expect(message).toBe(expected);
    });

    it("says the link leads nowhere when the slug is unknown or was rotated", () => {
        // given
        const error = makeApiError({ status: 404, message: "board not found" });

        // when
        const message = joinRefusal(error);

        // then
        expect(message).toContain("This link does not lead to a board.");
    });

    it("says to wait when the join limiter refuses", () => {
        // given
        const error = makeApiError({ status: 429, message: "too many requests, please try again later" });

        // when
        const message = joinRefusal(error);

        // then
        expect(message).toBe("Too many attempts to join. Wait a minute, then try again.");
    });

    it("leaves a password refusal to the password field", () => {
        // given
        const error = makeApiError({
            status: 403,
            message: "that is not the board's password",
            code: "wrong_password",
        });

        // when
        const message = joinRefusal(error);

        // then
        expect(message).toBeNull();
    });

    it("has nothing to say about an unknown failure, so the caller shows the server's own message", () => {
        // given
        const error = makeApiError({ status: 500, message: "internal error" });

        // when
        const message = joinRefusal(error);

        // then
        expect(message).toBeNull();
    });

    it("has nothing to say without an error", () => {
        // then
        expect(joinRefusal(null)).toBeNull();
    });
});

describe("joinPasswordRefusal", () => {
    it("puts a wrong password on the password field", () => {
        // given
        const error = makeApiError({
            status: 403,
            message: "that is not the board's password",
            code: "wrong_password",
        });

        // when
        const message = joinPasswordRefusal(error);

        // then
        expect(message).toBe("That is not the board's password.");
    });

    it("asks for the password when the board gained one during the join", () => {
        // given
        const error = makeApiError({ status: 403, message: "the board needs its password", code: "password_required" });

        // when
        const message = joinPasswordRefusal(error);

        // then
        expect(message).toBe("This board needs its password.");
    });

    it("ignores every other refusal", () => {
        // given
        const error = makeApiError({ status: 403, message: "removed", code: "removed" });

        // then
        expect(joinPasswordRefusal(error)).toBeNull();
    });
});

describe("needsAccount", () => {
    it.each(["account_required", "guests_not_allowed"])("says %s needs an account", code => {
        // given
        const error = makeApiError({ status: 403, message: "refused", code });

        // then
        expect(needsAccount(error)).toBe(true);
    });

    it("says nothing else needs an account", () => {
        // given
        const error = makeApiError({ status: 403, message: "refused", code: "link_disabled" });

        // then
        expect(needsAccount(error)).toBe(false);
        expect(needsAccount(null)).toBe(false);
    });
});

describe("sharedBoardRefusal", () => {
    it("tells a guest whose pass has gone to open the link again, not that a session expired", () => {
        // given
        const error = makeApiError({ status: 401, message: "invalid or expired session" });

        // when
        const message = sharedBoardRefusal(error, true);

        // then
        expect(message).toBe(
            "This browser no longer holds its guest pass for the board. Open the board's link again to rejoin.",
        );
    });

    it("asks a visitor with no pass to sign in or open the link", () => {
        // given
        const error = makeApiError({ status: 401, message: "authentication required" });

        // when
        const message = sharedBoardRefusal(error, false);

        // then
        expect(message).toBe("Sign in, or open the board's link, to see this board.");
    });

    it("says a private site needs a sign in or the link", () => {
        // given
        const error = makeApiError({
            status: 401,
            message: "this site is private, sign in to continue",
            code: "sign_in_required",
        });

        // when
        const message = sharedBoardRefusal(error, false);

        // then
        expect(message).toBe("This site is private. Sign in, or open the board's link again to join it.");
    });

    it("says the board is gone on a 404", () => {
        // given
        const error = makeApiError({ status: 404, message: "board not found" });

        // then
        expect(sharedBoardRefusal(error, true)).toBe(
            "This board is not here. It may have been deleted, or its link changed.",
        );
    });

    it("asks for the new password after it changed", () => {
        // given
        const error = makeApiError({ status: 403, message: "the board needs its password", code: "password_required" });

        // then
        expect(sharedBoardRefusal(error, true)).toBe(
            "This board's password changed. Open the board's link again and enter the new one.",
        );
    });

    it("has nothing to say about a failure that is not a refusal", () => {
        // given
        const error = makeApiError({ status: 503, message: "service unavailable" });

        // then
        expect(sharedBoardRefusal(error, true)).toBeNull();
        expect(sharedBoardRefusal(null, true)).toBeNull();
    });
});

describe("rejoinSlugFrom", () => {
    it("reads back the slug the join page left in the route state", () => {
        // when
        const slug = rejoinSlugFrom(rejoinState("k7Q2"));

        // then
        expect(slug).toBe("k7Q2");
    });

    it.each([null, undefined, "k7Q2", {}, { joinSlug: 7 }, { joinSlug: "" }])("finds no slug in %j", state => {
        // then
        expect(rejoinSlugFrom(state)).toBeNull();
    });
});
