import http from "k6/http";
import { check } from "k6";
import { Trend } from "k6/metrics";

const BASE_URL = __ENV.BASE_URL || "http://localhost:4333";

const LOAD_DURATION = "1m40s";

const ENDPOINTS = [
    { name: "livez", path: "/livez" },
    { name: "health", path: "/health" },
    { name: "landing", path: "/" },
];

const SERVER_SERIES = [
    { metric: "process_resident_memory_bytes", trend: new Trend("server_rss_mb"), scale: 1 / 1024 / 1024 },
    { metric: "go_memstats_alloc_bytes", trend: new Trend("server_heap_mb"), scale: 1 / 1024 / 1024 },
    { metric: "go_goroutines", trend: new Trend("server_goroutines"), scale: 1 },
    { metric: "process_open_fds", trend: new Trend("server_open_fds"), scale: 1 },
];

export const options = {
    scenarios: {
        load: {
            executor: "ramping-vus",
            startVUs: 0,
            stages: [
                { duration: "30s", target: 20 },
                { duration: "1m", target: 20 },
                { duration: "10s", target: 0 },
            ],
            gracefulRampDown: "30s",
            exec: "load",
        },
        sampler: {
            executor: "constant-arrival-rate",
            rate: 1,
            timeUnit: "1s",
            duration: LOAD_DURATION,
            preAllocatedVUs: 1,
            exec: "sampleServer",
        },
    },
    thresholds: {
        "http_req_failed{scenario:load}": ["rate<0.01"],
        "http_req_duration{endpoint:livez}": ["p(95)<100"],
        "http_req_duration{endpoint:health}": ["p(95)<300"],
        "http_req_duration{endpoint:landing}": ["p(95)<150"],
    },
};

export function load() {
    for (const endpoint of ENDPOINTS) {
        const res = http.get(`${BASE_URL}${endpoint.path}`, { tags: { endpoint: endpoint.name } });

        check(res, {
            [`${endpoint.name} is 200`]: r => r.status === 200,
            [`${endpoint.name} not host-rejected`]: r => r.status !== 403,
        });
    }
}

function promValue(body, name) {
    const match = new RegExp(`^${name}\\s+([0-9.eE+-]+)$`, "m").exec(body);
    if (!match) {
        return null;
    }

    return parseFloat(match[1]);
}

export function sampleServer() {
    const res = http.get(`${BASE_URL}/metrics`, { tags: { endpoint: "metrics-scrape" } });

    const scraped = check(res, { "metrics-scrape is 200": r => r.status === 200 });
    if (!scraped) {
        return;
    }

    for (const series of SERVER_SERIES) {
        const value = promValue(res.body, series.metric);

        const present = check(value, { [`metrics-scrape has ${series.metric}`]: v => v !== null });
        if (present) {
            series.trend.add(value * series.scale);
        }
    }
}
