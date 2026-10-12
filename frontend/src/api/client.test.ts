import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    ApiError,
    apiDelete,
    apiDeleteWithBody,
    apiFetch,
    apiPatch,
    apiPost,
    apiPut,
    apiUpload,
    buildQueryString,
} from "./client";
import { clearSessionLost, isSessionLost, setUserSessionHeld } from "./sessionLost";

function stubFetch(response: Response) {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(response);

    vi.stubGlobal("fetch", fetchMock);

    return fetchMock;
}

function jsonResponse(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
    });
}

beforeEach(() => {
    clearSessionLost();
    setUserSessionHeld(false);
});

describe("ApiError", () => {
    it("is an error that carries the status, message, code and raw body", () => {
        // given
        const body = { error: "the board is locked", code: "board_locked" };

        // when
        const error = new ApiError(409, "the board is locked", body);

        // then
        expect(error).toBeInstanceOf(Error);
        expect(error.name).toBe("ApiError");
        expect(error.status).toBe(409);
        expect(error.message).toBe("the board is locked");
        expect(error.code).toBe("board_locked");
        expect(error.body).toBe(body);
    });

    it("has no code when the body carries none", () => {
        // given
        const body = { error: "not found" };

        // when
        const error = new ApiError(404, "not found", body);

        // then
        expect(error.code).toBeNull();
    });

    it("has no code when the body's code is not a string", () => {
        // given
        const body = { error: "not allowed", code: 7 };

        // when
        const error = new ApiError(403, "not allowed", body);

        // then
        expect(error.code).toBeNull();
    });

    it("has no code when there is no body to read one from", () => {
        // given
        const bodies: unknown[] = [null, "gateway exploded", ["board_locked"]];

        // when
        const codes = bodies.map(body => new ApiError(500, "API error: 500", body).code);

        // then
        expect(codes).toEqual([null, null, null]);
    });
});

describe("apiFetch", () => {
    it("gets the versioned path with the client header and same-origin cookies, and returns the parsed body", async () => {
        // given
        const fetchMock = stubFetch(jsonResponse({ site_name: "RetroTide" }));

        // when
        const result = await apiFetch<{ site_name: string }>("/site-info");

        // then
        expect(fetchMock.mock.lastCall).toStrictEqual([
            "/api/v1/site-info",
            {
                method: "GET",
                headers: { "X-RetroTide-Client": "web" },
                credentials: "same-origin",
            },
        ]);
        expect(result).toEqual({ site_name: "RetroTide" });
    });

    it("resolves to undefined for a 204 no content response", async () => {
        // given
        stubFetch(new Response(null, { status: 204 }));

        // when
        const result = await apiFetch("/boards/b-1/participants/me");

        // then
        expect(result).toBeUndefined();
    });

    it("resolves to undefined when the response declares a zero content length", async () => {
        // given
        stubFetch(new Response("", { status: 200, headers: { "content-length": "0" } }));

        // when
        const result = await apiFetch("/boards/b-1/participants/me");

        // then
        expect(result).toBeUndefined();
    });

    it("throws an ApiError carrying the status, the server message, its code and the body", async () => {
        // given
        stubFetch(jsonResponse({ error: "the board is locked", code: "board_locked" }, 409));

        // when
        const failure = apiFetch("/boards/b-1");

        // then
        await expect(failure).rejects.toBeInstanceOf(ApiError);
        await expect(failure).rejects.toMatchObject({
            status: 409,
            message: "the board is locked",
            code: "board_locked",
            body: { error: "the board is locked", code: "board_locked" },
        });
    });

    it("marks the session lost when the server no longer accepts a signed in user's session", async () => {
        // given
        setUserSessionHeld(true);
        stubFetch(jsonResponse({ error: "authentication required" }, 401));

        // when
        const failure = apiPost("/auth/logout");

        // then
        await expect(failure).rejects.toMatchObject({ status: 401 });
        expect(isSessionLost()).toBe(true);
    });

    it("leaves the session alone on a 401 when no user session is held, because a guest's 401 means rejoin", async () => {
        // given
        setUserSessionHeld(false);
        stubFetch(jsonResponse({ error: "authentication required" }, 401));

        // when
        const failure = apiFetch("/boards/b-1");

        // then
        await expect(failure).rejects.toMatchObject({ status: 401 });
        expect(isSessionLost()).toBe(false);
    });

    it("leaves the session alone when a request fails for any other reason", async () => {
        // given
        setUserSessionHeld(true);
        stubFetch(jsonResponse({ error: "you may not do that" }, 403));

        // when
        const failure = apiFetch("/orgs/o-1/members");

        // then
        await expect(failure).rejects.toMatchObject({ status: 403 });
        expect(isSessionLost()).toBe(false);
    });

    it("keeps what a proxy said when it answers with an html error page", async () => {
        // given
        stubFetch(new Response("<html>gateway exploded</html>", { status: 502 }));

        // when
        const failure = apiFetch("/boards");

        // then
        await expect(failure).rejects.toBeInstanceOf(ApiError);
        await expect(failure).rejects.toMatchObject({
            status: 502,
            message: "API error: 502",
            code: null,
            body: "<html>gateway exploded</html>",
        });
    });

    it("keeps the raw text when the error body is json but not an object", async () => {
        // given
        stubFetch(jsonResponse(["the board is locked"], 500));

        // when
        const failure = apiFetch("/boards");

        // then
        await expect(failure).rejects.toMatchObject({
            status: 500,
            message: "API error: 500",
            code: null,
            body: '["the board is locked"]',
        });
    });

    it("keeps no body when the error response is empty", async () => {
        // given
        stubFetch(new Response(null, { status: 404 }));

        // when
        const failure = apiFetch("/boards/b-404");

        // then
        await expect(failure).rejects.toMatchObject({ status: 404, message: "API error: 404", code: null, body: null });
    });

    it("falls back to a generic message when the json error body has no error field", async () => {
        // given
        stubFetch(jsonResponse({ detail: "unprocessable" }, 422));

        // when
        const failure = apiFetch("/boards");

        // then
        await expect(failure).rejects.toMatchObject({
            status: 422,
            message: "API error: 422",
            code: null,
            body: { detail: "unprocessable" },
        });
    });
});

describe("apiPost", () => {
    it("posts a json body with the client header and same-origin cookies, and returns the parsed response", async () => {
        // given
        const fetchMock = stubFetch(jsonResponse({ id: "b-1" }));

        // when
        const result = await apiPost<{ id: string }, { title: string }>("/boards", { title: "Sprint review" });

        // then
        expect(fetchMock.mock.lastCall).toStrictEqual([
            "/api/v1/boards",
            {
                method: "POST",
                headers: { "X-RetroTide-Client": "web", "Content-Type": "application/json" },
                body: JSON.stringify({ title: "Sprint review" }),
                credentials: "same-origin",
            },
        ]);
        expect(result).toEqual({ id: "b-1" });
    });

    it("sends a bodiless write with the client header alone and no content type", async () => {
        // given
        const fetchMock = stubFetch(new Response(null, { status: 204 }));

        // when
        const result = await apiPost("/auth/logout");

        // then
        expect(fetchMock.mock.lastCall).toStrictEqual([
            "/api/v1/auth/logout",
            {
                method: "POST",
                headers: { "X-RetroTide-Client": "web" },
                credentials: "same-origin",
            },
        ]);
        expect(result).toBeUndefined();
    });

    it("throws an ApiError when the server rejects the body", async () => {
        // given
        stubFetch(jsonResponse({ error: "title is required", code: "validation_failed" }, 400));

        // when
        const failure = apiPost("/boards", {});

        // then
        await expect(failure).rejects.toMatchObject({
            status: 400,
            message: "title is required",
            code: "validation_failed",
        });
    });
});

describe("apiPut", () => {
    it("sends a json body with the PUT method, the client header and same-origin cookies", async () => {
        // given
        const fetchMock = stubFetch(jsonResponse({ id: "b-1" }));

        // when
        await apiPut<{ id: string }, { title: string }>("/boards/b-1", { title: "Revised" });

        // then
        expect(fetchMock.mock.lastCall).toStrictEqual([
            "/api/v1/boards/b-1",
            {
                method: "PUT",
                headers: { "X-RetroTide-Client": "web", "Content-Type": "application/json" },
                body: JSON.stringify({ title: "Revised" }),
                credentials: "same-origin",
            },
        ]);
    });
});

describe("apiPatch", () => {
    it("sends a json body with the PATCH method, the client header and same-origin cookies", async () => {
        // given
        const fetchMock = stubFetch(jsonResponse({ id: "b-1" }));

        // when
        await apiPatch<{ id: string }, { archived: boolean }>("/boards/b-1", { archived: true });

        // then
        expect(fetchMock.mock.lastCall).toStrictEqual([
            "/api/v1/boards/b-1",
            {
                method: "PATCH",
                headers: { "X-RetroTide-Client": "web", "Content-Type": "application/json" },
                body: JSON.stringify({ archived: true }),
                credentials: "same-origin",
            },
        ]);
    });
});

describe("apiDelete", () => {
    it("sends a DELETE with the client header and same-origin cookies, and no body or content type", async () => {
        // given
        const fetchMock = stubFetch(new Response(null, { status: 204 }));

        // when
        const result = await apiDelete("/boards/b-1");

        // then
        expect(fetchMock.mock.lastCall).toStrictEqual([
            "/api/v1/boards/b-1",
            {
                method: "DELETE",
                headers: { "X-RetroTide-Client": "web" },
                credentials: "same-origin",
            },
        ]);
        expect(result).toBeUndefined();
    });
});

describe("apiDeleteWithBody", () => {
    it("sends a DELETE that carries a json body beside the client header", async () => {
        // given
        const fetchMock = stubFetch(new Response(null, { status: 204 }));

        // when
        await apiDeleteWithBody<void, { ids: string[] }>("/notifications", { ids: ["n1", "n2"] });

        // then
        expect(fetchMock.mock.lastCall).toStrictEqual([
            "/api/v1/notifications",
            {
                method: "DELETE",
                headers: { "X-RetroTide-Client": "web", "Content-Type": "application/json" },
                body: JSON.stringify({ ids: ["n1", "n2"] }),
                credentials: "same-origin",
            },
        ]);
    });
});

describe("apiUpload", () => {
    it("posts the raw file with its own content type beside the client header", async () => {
        // given
        const fetchMock = stubFetch(jsonResponse({ id: "m-1" }, 201));
        const file = new Blob(["png-bytes"], { type: "image/png" });

        // when
        const result = await apiUpload<{ id: string }>("/boards/b-1/media?kind=card", file, "image/png");

        // then
        expect(fetchMock.mock.lastCall).toStrictEqual([
            "/api/v1/boards/b-1/media?kind=card",
            {
                method: "POST",
                headers: { "X-RetroTide-Client": "web", "Content-Type": "image/png" },
                body: file,
                credentials: "same-origin",
            },
        ]);
        expect(result).toEqual({ id: "m-1" });
    });

    it("throws an ApiError carrying the refusal code", async () => {
        // given
        stubFetch(
            jsonResponse({ error: "upgrade required", code: "upgrade_required", limit: "card_image_bytes" }, 402),
        );

        // when
        const failure = apiUpload("/boards/b-1/media?kind=card", new Blob(["x"]), "image/jpeg");

        // then
        await expect(failure).rejects.toMatchObject({
            status: 402,
            message: "upgrade required",
            code: "upgrade_required",
        });
    });
});

describe("defaulted request body generic", () => {
    it("accepts a single type argument on every body-carrying verb", async () => {
        // given
        const fetchMock = vi
            .fn<typeof fetch>()
            .mockImplementation(() => Promise.resolve(jsonResponse({ status: "ok" })));
        vi.stubGlobal("fetch", fetchMock);

        // when
        const posted = await apiPost<{ status: string }>("/boards", { title: "Sprint review" });
        const put = await apiPut<{ status: string }>("/boards/b-1", { title: "Sprint review" });
        const patched = await apiPatch<{ status: string }>("/boards/b-1", { title: "Sprint review" });
        const deleted = await apiDeleteWithBody<{ status: string }>("/boards/b-1", { reason: "duplicate" });

        // then
        expect(fetchMock).toHaveBeenCalledTimes(4);
        expect(fetchMock.mock.calls.map(call => call[1]?.method)).toEqual(["POST", "PUT", "PATCH", "DELETE"]);
        expect([posted, put, patched, deleted]).toEqual([
            { status: "ok" },
            { status: "ok" },
            { status: "ok" },
            { status: "ok" },
        ]);
    });
});

describe("buildQueryString", () => {
    it("returns an empty string when there is nothing to send", () => {
        // then
        expect(buildQueryString({})).toBe("");
        expect(buildQueryString({ q: undefined, cursor: "", offset: 0 })).toBe("");
    });

    it("skips undefined and empty string values", () => {
        // given
        const params = { q: "retro", cursor: "", limit: 20, sort: undefined };

        // when
        const qs = buildQueryString(params);

        // then
        expect(qs).toBe("?q=retro&limit=20");
    });

    it("drops a zero offset and page because zero means unset for paging", () => {
        // given
        const params = { q: "retro", offset: 0, page: 0, limit: 20 };

        // when
        const qs = buildQueryString(params);

        // then
        expect(qs).toBe("?q=retro&limit=20");
    });

    it("keeps a zero value for every other key", () => {
        // given
        const params = { limit: 0, votes: 0, seed: 0, perColumn: 0 };

        // when
        const qs = buildQueryString(params);

        // then
        expect(qs).toBe("?limit=0&votes=0&seed=0&perColumn=0");
    });

    it("keeps the declared parameter order and prefixes a question mark", () => {
        // then
        expect(buildQueryString({ limit: 20, offset: 40 })).toBe("?limit=20&offset=40");
    });

    it("percent-encodes keys and values", () => {
        // then
        expect(buildQueryString({ q: "a b&c=d" })).toBe("?q=a+b%26c%3Dd");
        expect(buildQueryString({ "a key": "v" })).toBe("?a+key=v");
    });

    it("stringifies numbers", () => {
        // then
        expect(buildQueryString({ limit: 20 })).toBe("?limit=20");
        expect(buildQueryString({ limit: -1 })).toBe("?limit=-1");
    });
});
