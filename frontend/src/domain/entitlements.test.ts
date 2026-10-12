import { describe, expect, it } from "vitest";
import { PLAN_LABELS, planAllowance, readUpgradeRequired } from "./entitlements";

function upgradeError(body: Record<string, unknown>, status = 402, code: string | null = "upgrade_required"): Error {
    return Object.assign(new Error("upgrade required"), { name: "ApiError", status, code, body });
}

const teamsBody = {
    error: "upgrade required",
    code: "upgrade_required",
    limit: "teams",
    allowed: 1,
    current: 1,
    plan: "free",
};

describe("readUpgradeRequired", () => {
    it("reads the limit, the allowance and the plan from a 402 upgrade_required answer", () => {
        // given
        const error = upgradeError(teamsBody);

        // when
        const upgrade = readUpgradeRequired(error);

        // then
        expect(upgrade).toEqual({
            error: "upgrade required",
            code: "upgrade_required",
            limit: "teams",
            allowed: 1,
            current: 1,
            plan: "free",
        });
    });

    it("reads nothing from any other failure", () => {
        // given
        const errors = [
            upgradeError(teamsBody, 403, null),
            upgradeError(teamsBody, 402, "payment_failed"),
            new Error("network down"),
            null,
        ];

        // when
        const upgrades = errors.map(error => readUpgradeRequired(error));

        // then
        expect(upgrades).toEqual([null, null, null, null]);
    });

    it("reads nothing from an answer whose body is not the upgrade shape, so its message is shown instead", () => {
        // given
        const errors = [
            upgradeError({ ...teamsBody, limit: "boards" }),
            upgradeError({ ...teamsBody, allowed: "1" }),
            upgradeError({ ...teamsBody, plan: "gold" }),
            Object.assign(new Error("upgrade required"), { status: 402, code: "upgrade_required", body: "text" }),
        ];

        // when
        const upgrades = errors.map(error => readUpgradeRequired(error));

        // then
        expect(upgrades).toEqual([null, null, null, null]);
    });
});

describe("planAllowance", () => {
    it("says how many teams the plan allows, in the singular for one", () => {
        // when
        const sentence = planAllowance({ plan: "free", limit: "teams", allowed: 1 });

        // then
        expect(sentence).toBe("The free plan allows 1 team.");
    });

    it("says how many members the plan allows", () => {
        // when
        const sentence = planAllowance({ plan: "free", limit: "members", allowed: 10 });

        // then
        expect(sentence).toBe("The free plan allows 10 members.");
    });

    it("says how many saved templates the plan allows", () => {
        // when
        const sentence = planAllowance({ plan: "free", limit: "saved_templates", allowed: 3 });

        // then
        expect(sentence).toBe("The free plan allows 3 saved templates.");
    });

    it("says how large an uploaded image the plan allows", () => {
        // when
        const sentence = planAllowance({ plan: "free", limit: "card_image_bytes", allowed: 5242880 });

        // then
        expect(sentence).toBe("The free plan allows 5242880 bytes per image.");
    });
});

describe("readUpgradeRequired for saved templates", () => {
    it("reads a refusal of a fourth saved template", () => {
        // given
        const error = upgradeError({ ...teamsBody, limit: "saved_templates", allowed: 3, current: 3 });

        // when
        const upgrade = readUpgradeRequired(error);

        // then
        expect(upgrade?.limit).toBe("saved_templates");
        expect(upgrade?.allowed).toBe(3);
    });
});

describe("PLAN_LABELS", () => {
    it("names every plan for a heading or a table cell", () => {
        // then
        expect(PLAN_LABELS).toEqual({ free: "Free" });
    });
});
