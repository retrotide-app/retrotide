import { describe, expect, it } from "vitest";
import { makeApiError, makeBoardTemplate } from "../test-utils/fixtures";
import {
    columnCountLabel,
    isPublicTemplate,
    isTemplateNotFound,
    publicTemplates,
    templatePath,
    templateStartPath,
} from "./templates";

describe("publicTemplates", () => {
    it("leaves the blank template out and keeps the catalogue order", () => {
        // given
        const sailboat = makeBoardTemplate({ key: "sailboat", name: "Sailboat" });
        const starfish = makeBoardTemplate({ key: "starfish", name: "Starfish" });
        const blank = makeBoardTemplate();

        // when
        const shown = publicTemplates([sailboat, blank, starfish]);

        // then
        expect(shown).toEqual([sailboat, starfish]);
    });
});

describe("isPublicTemplate", () => {
    it("tells the blank template from a public one", () => {
        // when
        const verdicts = [isPublicTemplate("sailboat"), isPublicTemplate("blank")];

        // then
        expect(verdicts).toEqual([true, false]);
    });
});

describe("templatePath", () => {
    it("puts a template's page under /templates and escapes its key", () => {
        // when
        const paths = [templatePath("sailboat"), templatePath("a/b")];

        // then
        expect(paths).toEqual(["/templates/sailboat", "/templates/a%2Fb"]);
    });
});

describe("isTemplateNotFound", () => {
    it("treats only a 404 as an unknown template", () => {
        // given
        const missing = makeApiError({ status: 404, message: "template not found" });
        const broken = makeApiError({ status: 500, message: "internal error" });

        // when
        const verdicts = [isTemplateNotFound(missing), isTemplateNotFound(broken), isTemplateNotFound(null)];

        // then
        expect(verdicts).toEqual([true, false, false]);
    });
});

describe("columnCountLabel", () => {
    it("counts one column and many columns", () => {
        // when
        const labels = [columnCountLabel(1), columnCountLabel(4)];

        // then
        expect(labels).toEqual(["1 column", "4 columns"]);
    });
});

describe("templateStartPath", () => {
    it("sends a visitor to sign up, a member to a new board, and someone with no current organisation to the front door", () => {
        // when
        const paths = [
            templateStartPath(false, null),
            templateStartPath(true, "harbour"),
            templateStartPath(true, null),
        ];

        // then the front door decides between onboarding, a pending invite and the staff console
        expect(paths).toEqual(["/register", "/o/harbour/boards/new", "/"]);
    });
});
