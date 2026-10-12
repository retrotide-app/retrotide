const commentMessage =
    "Code carries no comments. Say it in a name or a test, or delete it. A test keeps only its given / when / then markers.";
const unreasonedDisableMessage =
    "An oxlint-disable-next-line names each rule it switches off and states why after a ` -- `, as in `oxlint-disable-next-line react/exhaustive-deps -- the reason`.";
const layerDisableMessage =
    "A layers/ rule is never switched off inline. Fix the file, or change the layer map in oxlint.layers.ts.";
const broadDisableMessage =
    "Only a reasoned oxlint-disable-next-line may switch a rule off. A directive that covers its own line or the rest of the file is not allowed.";

const disableDirective = "oxlint-disable-next-line";
const reasonSeparator = " -- ";
const testFilePattern = /(\.test\.[cm]?[jt]sx?|[\\/]src[\\/]api[\\/]endpoints[\\/]testHarness\.ts)$/;
const markerPattern = /^(given|when|then)(\s*\/\s*(given|when|then))*(\s.*)?$/;
const broadDisablePattern = /^(?:eslint|oxlint)-disable(?<scope>-line)?(?:\s|$)/;

const isMarker = (comment, text, inTest) => inTest && comment.type === "Line" && markerPattern.test(text);

const checkDisable = (context, comment, text) => {
    const separatorAt = text.indexOf(reasonSeparator);
    const directive = separatorAt === -1 ? text : text.slice(0, separatorAt);
    const reason = separatorAt === -1 ? "" : text.slice(separatorAt + reasonSeparator.length).trim();

    const rules = directive
        .slice(disableDirective.length)
        .split(/[\s,]+/)
        .filter(rule => rule !== "");

    if (rules.some(rule => rule.startsWith("layers/"))) {
        context.report({ node: comment, messageId: "layerDisable" });
        return;
    }

    if (rules.length === 0 || reason === "") {
        context.report({ node: comment, messageId: "unreasonedDisable" });
    }
};

const reportBroadDisable = (context, comment, scope) => {
    const [start, end] = comment.range;
    const outsideItsOwnReach = scope === undefined ? start : end;

    context.report({ loc: context.sourceCode.getLocFromIndex(outsideItsOwnReach), messageId: "broadDisable" });
};

const checkComment = (context, comment, inTest) => {
    const text = comment.value.trim();

    if (isMarker(comment, text, inTest)) {
        return;
    }

    if (text === disableDirective || text.startsWith(`${disableDirective} `)) {
        checkDisable(context, comment, text);
        return;
    }

    const broad = broadDisablePattern.exec(text);

    if (broad) {
        reportBroadDisable(context, comment, broad.groups.scope);
        return;
    }

    context.report({ node: comment, messageId: "comment" });
};

const noComments = {
    meta: {
        type: "suggestion",
        messages: {
            comment: commentMessage,
            unreasonedDisable: unreasonedDisableMessage,
            layerDisable: layerDisableMessage,
            broadDisable: broadDisableMessage,
        },
    },
    create(context) {
        const inTest = testFilePattern.test(context.filename);

        return {
            Program() {
                for (const comment of context.sourceCode.getAllComments()) {
                    checkComment(context, comment, inTest);
                }
            },
        };
    },
};

export default {
    meta: { name: "house" },
    rules: {
        "no-comments": noComments,
    },
};
