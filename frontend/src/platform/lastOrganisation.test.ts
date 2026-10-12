import { afterEach, describe, expect, it, vi } from "vitest";
import { setPlatformErrorReporter } from "./errorReporter";
import { LAST_ORGANISATION_STORAGE_KEY, readLastOrganisation, writeLastOrganisation } from "./lastOrganisation";

afterEach(() => {
    vi.restoreAllMocks();
});

describe("LAST_ORGANISATION_STORAGE_KEY", () => {
    it("keeps the key the chosen organisation was saved under", () => {
        // given
        const expected = "rt-last-org";

        // when
        const key = LAST_ORGANISATION_STORAGE_KEY;

        // then
        expect(key).toBe(expected);
    });
});

describe("readLastOrganisation", () => {
    it("finds no organisation when none has been chosen in this browser", () => {
        // given
        localStorage.removeItem(LAST_ORGANISATION_STORAGE_KEY);

        // when
        const orgId = readLastOrganisation();

        // then
        expect(orgId).toBeNull();
    });

    it("reads back the organisation chosen last", () => {
        // given
        writeLastOrganisation("o1");
        writeLastOrganisation("o2");

        // when
        const orgId = readLastOrganisation();

        // then
        expect(orgId).toBe("o2");
    });

    it("treats an empty stored value as no choice", () => {
        // given
        localStorage.setItem(LAST_ORGANISATION_STORAGE_KEY, "");

        // when
        const orgId = readLastOrganisation();

        // then
        expect(orgId).toBeNull();
    });

    it("reports a storage read that throws and finds no organisation", () => {
        // given
        const report = vi.fn();
        setPlatformErrorReporter(report);
        const failure = new DOMException("site data is disabled", "SecurityError");
        vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
            throw failure;
        });

        // when
        const orgId = readLastOrganisation();

        // then
        expect(orgId).toBeNull();
        expect(report).toHaveBeenCalledWith(failure, { source: "storage" });
    });
});

describe("writeLastOrganisation", () => {
    it("stores the organisation under its key in local storage, so it outlives the tab", () => {
        // given
        localStorage.removeItem(LAST_ORGANISATION_STORAGE_KEY);

        // when
        writeLastOrganisation("o1");

        // then
        expect(localStorage.getItem(LAST_ORGANISATION_STORAGE_KEY)).toBe("o1");
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
        const attempt = () => writeLastOrganisation("o1");

        // then
        expect(attempt).not.toThrow();
        expect(report).toHaveBeenCalledWith(failure, { source: "storage" });
    });
});
