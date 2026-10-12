import { describe, expect, it } from "vitest";
import { makeApiError, makeBoardTemplate, makeSavedTemplate } from "../test-utils/fixtures";
import {
    BUILT_IN_TEMPLATES_GROUP_LABEL,
    builtInOption,
    newBoardFromTemplatePath,
    organisationTemplatesPath,
    pickedTemplateOption,
    SAVED_TEMPLATES_GROUP_LABEL,
    savedOption,
    savedTemplateNote,
    savedTemplateRefusal,
    savedTemplateValue,
    templateFields,
    templateGroups,
} from "./savedTemplates";

const blank = makeBoardTemplate();
const sailboat = makeBoardTemplate({ key: "sailboat", name: "Sailboat" });
const mine = makeSavedTemplate({ id: "s1", name: "Mine" });
const shared = makeSavedTemplate({ id: "s2", name: "Shared", scope: "team", team_id: "t1", team_name: "Platform" });

describe("organisationTemplatesPath", () => {
    it("addresses the organisation's templates page", () => {
        // when
        const path = organisationTemplatesPath("harbour docks");

        // then
        expect(path).toBe("/o/harbour%20docks/templates");
    });
});

describe("newBoardFromTemplatePath", () => {
    it("opens the new board page with the saved template preselected", () => {
        // when
        const path = newBoardFromTemplatePath("harbour", "s1");

        // then
        expect(path).toBe("/o/harbour/boards/new?saved=s1");
    });
});

describe("savedTemplateNote", () => {
    it("says who a saved template is for", () => {
        // when
        const notes = [
            savedTemplateNote(mine),
            savedTemplateNote(shared),
            savedTemplateNote(makeSavedTemplate({ scope: "team" })),
        ];

        // then
        expect(notes).toEqual(["Only you", "Shared with Platform", "Shared with a team"]);
    });
});

describe("template options", () => {
    it("keeps a built-in key and a saved id apart in the option value", () => {
        // when
        const options = [builtInOption(sailboat), savedOption(mine)];

        // then
        expect(options.map(option => option.value)).toEqual(["builtin:sailboat", savedTemplateValue("s1")]);
        expect(options.map(option => option.choice)).toEqual([
            { kind: "builtin", key: "sailboat" },
            { kind: "saved", id: "s1" },
        ]);
        expect(options[1].note).toBe("Only you");
        expect(options[0].note).toBeNull();
    });

    it("puts your templates above the built-in ones", () => {
        // when
        const groups = templateGroups([mine, shared], [sailboat, blank]);

        // then
        expect(groups.map(group => group.label)).toEqual([SAVED_TEMPLATES_GROUP_LABEL, BUILT_IN_TEMPLATES_GROUP_LABEL]);
        expect(groups[0].options.map(option => option.name)).toEqual(["Mine", "Shared"]);
    });

    it("leaves out an empty group", () => {
        // when
        const groups = templateGroups([], [sailboat]);

        // then
        expect(groups.map(group => group.label)).toEqual([BUILT_IN_TEMPLATES_GROUP_LABEL]);
    });
});

describe("pickedTemplateOption", () => {
    const groups = templateGroups([mine], [sailboat, blank]);

    it("finds the picked option", () => {
        // when
        const picked = pickedTemplateOption(groups, savedTemplateValue("s1"));

        // then
        expect(picked?.name).toBe("Mine");
    });

    it("falls back to the first built-in template when nothing or something unknown is picked", () => {
        // when
        const picked = [pickedTemplateOption(groups, null), pickedTemplateOption(groups, "saved:gone")];

        // then
        expect(picked.map(option => option?.name)).toEqual(["Sailboat", "Sailboat"]);
    });

    it("finds nothing without templates", () => {
        // when
        const picked = pickedTemplateOption([], null);

        // then
        expect(picked).toBeNull();
    });
});

describe("templateFields", () => {
    it("sends a built-in key or a saved template id", () => {
        // when
        const fields = [
            templateFields({ kind: "builtin", key: "sailboat" }),
            templateFields({ kind: "saved", id: "s1" }),
        ];

        // then
        expect(fields).toEqual([{ template_key: "sailboat" }, { template_key: "", saved_template_id: "s1" }]);
    });
});

describe("savedTemplateRefusal", () => {
    it("ties each refusal to its field", () => {
        // given
        const tooLong = makeApiError({ status: 400, message: "too long", code: "template_name_too_long" });
        const notTeam = makeApiError({ status: 409, message: "not a team board", code: "not_team_board" });

        // when
        const refusals = [
            savedTemplateRefusal(tooLong, "name"),
            savedTemplateRefusal(tooLong, "description"),
            savedTemplateRefusal(notTeam, "scope"),
            savedTemplateRefusal(null, "name"),
        ];

        // then
        expect(refusals).toEqual(["too long", null, "not a team board", null]);
    });
});
