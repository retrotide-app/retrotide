const rawQueryKeyMessage =
    "Query keys are built in src/api/queryKeys.ts only. Call the builder instead of writing a key array.";
const hoistedQueryKeyMessage =
    "An `as const` array hoisted outside src/api/queryKeys.ts is a query key built beside its builder. Move a key into src/api/queryKeys.ts, and give any other list a readonly type annotation instead of `as const`.";
const dynamicApiImportMessage =
    "Reaching src/api through import() bypasses the layer rules. Import it in the layer that is allowed to.";
const stubGlobalFetchMessage =
    "Only the transport layer's own test may stub global fetch. A test above the transport mocks the module it calls instead of the network.";
const renderTestTransportMockMessage =
    "A render test may not mock api/endpoints, api/queryKeys, api/client or api/queryClient. Mock the hook in src/hooks that the component calls, so the test states the same contract the component is allowed to depend on.";
const pureTestRenderMessage =
    "A test under domain or utils may not import @testing-library/react. A pure module takes values and returns values, so its test needs no DOM.";
const dndImportMessage =
    "@dnd-kit is imported only under src/components/board. Dragging is a render concern: the board components turn a drop into a {column_id, after_card_id} intent and hand it to a callback prop.";

const queryClientMethods =
    "setQueryData|getQueryData|setQueriesData|getQueriesData|invalidateQueries|removeQueries|cancelQueries|resetQueries|refetchQueries|query";
const asConst = "TSAsExpression[typeAnnotation.typeName.name='const']";

const rawQueryKeySelectors = [
    `CallExpression[callee.property.name=/^(${queryClientMethods})$/] > ArrayExpression:first-child`,
    `CallExpression[callee.property.name=/^(${queryClientMethods})$/] > ${asConst}:first-child > ArrayExpression`,
    "Property[key.name='queryKey'] > ArrayExpression > Literal:first-child",
    `Property[key.name='queryKey'] > ${asConst} > ArrayExpression > Literal:first-child`,
];
const hoistedQueryKeySelectors = [
    `Program > VariableDeclaration > VariableDeclarator > ${asConst} > ArrayExpression`,
    `Program > ExportNamedDeclaration > VariableDeclaration > VariableDeclarator > ${asConst} > ArrayExpression`,
];
const dynamicApiImportSelectors = [
    "TSImportType[source.value=/(^|\\/)api\\//]",
    "ImportExpression[source.value=/(^|\\/)api\\//]",
];
const stubGlobalFetchSelectors = [
    "CallExpression[callee.object.name='vi'][callee.property.name='stubGlobal'] > Literal[value='fetch']:first-child",
];
const renderTestTransportMockSelectors = [
    "CallExpression[callee.object.name='vi'][callee.property.name='mock'] > Literal[value=/api\\/(endpoints|queryKeys|client|queryClient)/]:first-child",
];
const pureTestRenderSelectors = ["ImportDeclaration[source.value=/^@testing-library\\/react/]"];
const dndImportSelectors = [
    "ImportDeclaration[source.value=/^@dnd-kit\\//]",
    "ExportNamedDeclaration[source.value=/^@dnd-kit\\//]",
    "ExportAllDeclaration[source.value=/^@dnd-kit\\//]",
    "ImportExpression[source.value=/^@dnd-kit\\//]",
    "TSImportType[source.value=/^@dnd-kit\\//]",
];

const restrict = (...groups) => ({
    meta: {
        type: "problem",
        messages: Object.fromEntries(groups.map(group => [group.id, group.message])),
    },
    create(context) {
        const visitor = {};

        for (const group of groups) {
            for (const selector of group.selectors) {
                visitor[selector] = node => context.report({ node, messageId: group.id });
            }
        }

        return visitor;
    },
});

export default {
    meta: { name: "layers" },
    rules: {
        "no-raw-query-key": restrict(
            { id: "raw", message: rawQueryKeyMessage, selectors: rawQueryKeySelectors },
            { id: "hoisted", message: hoistedQueryKeyMessage, selectors: hoistedQueryKeySelectors },
        ),
        "no-dynamic-api-import": restrict({
            id: "restricted",
            message: dynamicApiImportMessage,
            selectors: dynamicApiImportSelectors,
        }),
        "no-stub-global-fetch": restrict({
            id: "restricted",
            message: stubGlobalFetchMessage,
            selectors: stubGlobalFetchSelectors,
        }),
        "no-transport-mock-in-render-test": restrict({
            id: "restricted",
            message: renderTestTransportMockMessage,
            selectors: renderTestTransportMockSelectors,
        }),
        "no-render-import-in-pure-test": restrict({
            id: "restricted",
            message: pureTestRenderMessage,
            selectors: pureTestRenderSelectors,
        }),
        "no-dnd-import": restrict({
            id: "restricted",
            message: dndImportMessage,
            selectors: dndImportSelectors,
        }),
    },
};
