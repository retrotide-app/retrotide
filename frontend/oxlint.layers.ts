import type { OxlintOverride } from "oxlint";

const RENDER = ["src/components/**/*.{ts,tsx}", "src/pages/**/*.{ts,tsx}", "src/App.tsx", "src/App.test.tsx"];
const ORCHESTRATION = ["src/hooks/**/*.{ts,tsx}", "src/context/**/*.{ts,tsx}"];
const DATA_HOOKS = ["src/hooks/queries/**/*.{ts,tsx}", "src/hooks/mutations/**/*.{ts,tsx}"];
const PURE = ["src/domain/**/*.{ts,tsx}", "src/utils/**/*.{ts,tsx}", "src/types/**/*.{ts,tsx}"];
const API = ["src/api/**/*.{ts,tsx}"];
const PLATFORM = ["src/platform/**/*.{ts,tsx}"];
const ADAPTERS = [...API, ...PLATFORM];
const COMPOSITION_ROOT = ["src/main.tsx"];
const EVERY_SOURCE_FILE = ["src/**/*.{ts,tsx}"];
const RENDER_TESTS = ["src/components/**/*.test.{ts,tsx}", "src/pages/**/*.test.{ts,tsx}", "src/App.test.tsx"];
const PURE_TESTS = ["src/domain/**/*.test.{ts,tsx}", "src/utils/**/*.test.{ts,tsx}"];
const TRANSPORT_TEST = ["src/api/client.test.ts"];
const QUERY_KEY_BUILDERS = ["src/api/queryKeys.ts"];
const DND_IMPORTERS = ["src/components/board/**/*.{ts,tsx}"];

const apiAny = ["**/api/*", "**/api/**"];
const apiEndpoints = ["**/api/endpoints", "**/api/endpoints.ts", "**/api/endpoints/**"];
const apiTransport = ["**/api/client", "**/api/client.ts", "**/api/queryClient", "**/api/queryClient.ts"];
const reactRuntime = ["react", "react-dom", "@tanstack/react-query"];
const reactQuery = ["@tanstack/react-query"];
const renderAny = ["**/components/**", "**/pages/**", "**/App", "**/App.tsx"];
const contextAny = ["**/context/*", "**/context/**"];
const upward = [...renderAny, "**/hooks/**", ...contextAny];
const platformAny = ["**/platform/*", "**/platform/**"];

const renderApiMessage =
    "The render layer may not import src/api. Read the data from a hook in src/hooks and destructure it.";
const renderQueryMessage =
    "The render layer may not import react-query. Wrap the query in a data hook under src/hooks/queries or src/hooks/mutations.";
const transportMessage =
    "Only src/api, src/hooks/queries, src/hooks/mutations and src/main.tsx may import the transport modules api/client, api/queryClient and api/endpoints.";
const pureApiMessage = "The pure layer may not import src/api. A pure module takes values and returns values.";
const pureReactMessage =
    "The pure layer may not import react, react-dom or react-query. A module that needs them belongs in src/hooks.";
const pureUpwardMessage =
    "The pure layer may not import components, pages, App, hooks or context. A pure module takes values and returns values, so pass it what it needs.";
const purePlatformMessage =
    "The pure layer may not import src/platform. A pure module never touches the browser, so the caller reads the device and passes the value in.";
const orchestrationRenderMessage =
    "src/hooks and src/context may not import components, pages or App. Orchestration hands data and callbacks down to the render layer, never the other way.";
const dataHookUpwardMessage =
    "A data hook may not import components, pages, App or context. It wraps one query or mutation and returns its result.";
const dataHookPlatformMessage =
    "A data hook may not import src/platform. The orchestration hook that calls it reads the device and passes the value in.";
const adapterUpwardMessage =
    "The adapter layers may not import upward from components, pages, App, hooks or context. Inject what the adapter needs.";
const platformApiMessage =
    "src/platform may not import src/api. The server call is passed in as a parameter by the caller.";
const apiPlatformMessage =
    "src/api may not import src/platform. Ask the browser API directly, or have the caller inject what the adapter needs.";

export const layerRules: OxlintOverride[] = [
    {
        files: DATA_HOOKS,
        rules: {
            "no-restricted-imports": [
                "error",
                {
                    patterns: [
                        { group: [...renderAny, ...contextAny], message: dataHookUpwardMessage, caseSensitive: true },
                        { group: platformAny, message: dataHookPlatformMessage },
                    ],
                },
            ],
        },
    },
    {
        files: ORCHESTRATION,
        excludeFiles: DATA_HOOKS,
        rules: {
            "no-restricted-imports": [
                "error",
                {
                    patterns: [
                        { group: [...apiTransport, ...apiEndpoints], message: transportMessage },
                        { group: renderAny, message: orchestrationRenderMessage, caseSensitive: true },
                    ],
                },
            ],
        },
    },
    {
        files: RENDER,
        rules: {
            "no-restricted-imports": [
                "error",
                {
                    patterns: [
                        { group: apiAny, message: renderApiMessage, allowTypeImports: false },
                        { group: reactQuery, message: renderQueryMessage, allowTypeImports: false },
                    ],
                },
            ],
        },
    },
    {
        files: PURE,
        rules: {
            "no-restricted-imports": [
                "error",
                {
                    patterns: [
                        { group: apiAny, message: pureApiMessage },
                        { group: reactRuntime, message: pureReactMessage },
                        { group: upward, message: pureUpwardMessage, caseSensitive: true },
                        { group: platformAny, message: purePlatformMessage },
                    ],
                },
            ],
        },
    },
    {
        files: API,
        rules: {
            "no-restricted-imports": [
                "error",
                {
                    patterns: [
                        { group: upward, message: adapterUpwardMessage, caseSensitive: true },
                        { group: platformAny, message: apiPlatformMessage },
                    ],
                },
            ],
        },
    },
    {
        files: PLATFORM,
        rules: {
            "no-restricted-imports": [
                "error",
                {
                    patterns: [
                        { group: upward, message: adapterUpwardMessage, caseSensitive: true },
                        { group: apiAny, message: platformApiMessage },
                    ],
                },
            ],
        },
    },
    {
        files: EVERY_SOURCE_FILE,
        excludeFiles: [...RENDER, ...DATA_HOOKS, ...ORCHESTRATION, ...PURE, ...ADAPTERS, ...COMPOSITION_ROOT],
        rules: {
            "no-restricted-imports": [
                "error",
                {
                    patterns: [{ group: [...apiTransport, ...apiEndpoints], message: transportMessage }],
                },
            ],
        },
    },
    {
        files: EVERY_SOURCE_FILE,
        rules: {
            "layers/no-raw-query-key": "error",
            "layers/no-dynamic-api-import": "error",
            "layers/no-stub-global-fetch": "error",
            "layers/no-dnd-import": "error",
        },
    },
    {
        files: DND_IMPORTERS,
        rules: {
            "layers/no-dnd-import": "off",
        },
    },
    {
        files: RENDER_TESTS,
        rules: {
            "layers/no-transport-mock-in-render-test": "error",
        },
    },
    {
        files: PURE_TESTS,
        rules: {
            "layers/no-render-import-in-pure-test": "error",
        },
    },
    {
        files: TRANSPORT_TEST,
        rules: {
            "layers/no-stub-global-fetch": "off",
        },
    },
    {
        files: QUERY_KEY_BUILDERS,
        rules: {
            "layers/no-raw-query-key": "off",
        },
    },
];
