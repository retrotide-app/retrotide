import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { RootErrorBoundary } from "./RootErrorBoundary";

let thrown: unknown = null;

function Fragile() {
    if (thrown !== null) {
        throw thrown;
    }

    return <p>The board is still here</p>;
}

beforeEach(() => {
    thrown = null;
    vi.spyOn(console, "error").mockReturnValue(undefined);
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe("RootErrorBoundary", () => {
    it("stays out of the way while nothing has gone wrong", () => {
        // given
        const intact = <Fragile />;

        // when
        render(<RootErrorBoundary>{intact}</RootErrorBoundary>);

        // then
        expect(screen.getByText("The board is still here")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Something went wrong" })).not.toBeInTheDocument();
    });

    it("shows a fallback instead of a blank page when a child throws", () => {
        // given
        thrown = new Error("the board could not be drawn");

        // when
        render(
            <RootErrorBoundary>
                <Fragile />
            </RootErrorBoundary>,
        );

        // then
        expect(screen.getByRole("heading", { name: "Something went wrong" })).toBeInTheDocument();
        expect(screen.queryByText("The board is still here")).not.toBeInTheDocument();
    });

    it("shows what the error said so the reader can quote it in a report", () => {
        // given
        thrown = new TypeError("columns is not iterable");

        // when
        render(
            <RootErrorBoundary>
                <Fragile />
            </RootErrorBoundary>,
        );

        // then
        expect(screen.getByText("TypeError: columns is not iterable")).toBeInTheDocument();
    });

    it("names an error that carries no message", () => {
        // given
        thrown = new RangeError("");

        // when
        render(
            <RootErrorBoundary>
                <Fragile />
            </RootErrorBoundary>,
        );

        // then
        expect(screen.getByText("RangeError")).toBeInTheDocument();
    });

    it("shows a thrown string as it was thrown", () => {
        // given
        thrown = "the column vanished";

        // when
        render(
            <RootErrorBoundary>
                <Fragile />
            </RootErrorBoundary>,
        );

        // then
        expect(screen.getByText("the column vanished")).toBeInTheDocument();
    });

    it("says nothing about a thrown value it cannot describe", () => {
        // given
        thrown = 42;

        // when
        render(
            <RootErrorBoundary>
                <Fragile />
            </RootErrorBoundary>,
        );

        // then
        expect(screen.getByRole("heading", { name: "Something went wrong" })).toBeInTheDocument();
        expect(screen.queryByText("42")).not.toBeInTheDocument();
    });

    it("gives the reader their work back when they try again", async () => {
        // given
        thrown = new Error("the board could not be drawn");
        const user = userEvent.setup();
        render(
            <RootErrorBoundary>
                <Fragile />
            </RootErrorBoundary>,
        );

        // when
        thrown = null;
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByText("The board is still here")).toBeInTheDocument();
        expect(screen.queryByRole("heading", { name: "Something went wrong" })).not.toBeInTheDocument();
    });

    it("offers a reload for when trying again is not enough", () => {
        // given
        thrown = new Error("the board could not be drawn");

        // when
        render(
            <RootErrorBoundary>
                <Fragile />
            </RootErrorBoundary>,
        );

        // then
        expect(screen.getByRole("button", { name: "Reload the page" })).toBeInTheDocument();
    });

    it("offers the way back to the home page", () => {
        // given
        thrown = new Error("the board could not be drawn");

        // when
        render(
            <RootErrorBoundary>
                <Fragile />
            </RootErrorBoundary>,
        );

        // then
        expect(screen.getByRole("link", { name: "Back to RetroTide" })).toHaveAttribute("href", "/");
    });
});
