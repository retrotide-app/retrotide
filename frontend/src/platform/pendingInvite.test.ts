import { afterEach, describe, expect, it, vi } from "vitest";
import { setPlatformErrorReporter } from "./errorReporter";
import { clearPendingInvite, PENDING_INVITE_STORAGE_KEY, readPendingInvite, writePendingInvite } from "./pendingInvite";

afterEach(() => {
    vi.restoreAllMocks();
    sessionStorage.clear();
});

describe("PENDING_INVITE_STORAGE_KEY", () => {
    it("keeps the key a pending invite was saved under", () => {
        // given
        const expected = "rt-pending-invite";

        // when
        const key = PENDING_INVITE_STORAGE_KEY;

        // then
        expect(key).toBe(expected);
    });
});

describe("readPendingInvite", () => {
    it("finds no invite when none is pending", () => {
        // given
        sessionStorage.removeItem(PENDING_INVITE_STORAGE_KEY);

        // when
        const token = readPendingInvite();

        // then
        expect(token).toBeNull();
    });

    it("reads back the invite's token", () => {
        // given
        writePendingInvite("invite-1");

        // when
        const token = readPendingInvite();

        // then
        expect(token).toBe("invite-1");
    });

    it("finds no invite once it has been cleared", () => {
        // given
        writePendingInvite("invite-1");
        clearPendingInvite();

        // when
        const token = readPendingInvite();

        // then
        expect(token).toBeNull();
    });

    it("treats an empty stored value as no invite", () => {
        // given
        sessionStorage.setItem(PENDING_INVITE_STORAGE_KEY, "");

        // when
        const token = readPendingInvite();

        // then
        expect(token).toBeNull();
    });

    it("reports a storage read that throws and finds no invite", () => {
        // given
        const report = vi.fn();
        setPlatformErrorReporter(report);
        const failure = new DOMException("site data is disabled", "SecurityError");
        vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
            throw failure;
        });

        // when
        const token = readPendingInvite();

        // then
        expect(token).toBeNull();
        expect(report).toHaveBeenCalledWith(failure, { source: "storage" });
    });
});

describe("writePendingInvite", () => {
    it("keeps the token in session storage, so it lasts for the tab and no longer", () => {
        // given
        sessionStorage.removeItem(PENDING_INVITE_STORAGE_KEY);

        // when
        writePendingInvite("invite-1");

        // then
        expect(sessionStorage.getItem(PENDING_INVITE_STORAGE_KEY)).toBe("invite-1");
        expect(localStorage.getItem(PENDING_INVITE_STORAGE_KEY)).toBeNull();
    });

    it("reports a storage write that throws and carries on", () => {
        // given
        const report = vi.fn();
        setPlatformErrorReporter(report);
        const failure = new DOMException("the quota is exhausted", "QuotaExceededError");
        vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
            throw failure;
        });

        // when
        const attempt = () => writePendingInvite("invite-1");

        // then
        expect(attempt).not.toThrow();
        expect(report).toHaveBeenCalledWith(failure, { source: "storage" });
    });
});

describe("clearPendingInvite", () => {
    it("reports a storage removal that throws and carries on", () => {
        // given
        const report = vi.fn();
        setPlatformErrorReporter(report);
        const failure = new DOMException("site data is disabled", "SecurityError");
        vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => {
            throw failure;
        });

        // when
        const attempt = clearPendingInvite;

        // then
        expect(attempt).not.toThrow();
        expect(report).toHaveBeenCalledWith(failure, { source: "storage" });
    });
});
