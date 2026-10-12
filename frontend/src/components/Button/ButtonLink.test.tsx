import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Button } from "./Button";
import { ButtonLink } from "./ButtonLink";

function StateProbe() {
    const location = useLocation();

    return <p>{`Arrived with ${JSON.stringify(location.state)}`}</p>;
}

describe("ButtonLink", () => {
    it("hands the route state it is given to the page it opens", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(
            <Routes>
                <Route
                    path="/"
                    element={
                        <ButtonLink to="/login" state={{ from: "/invite" }}>
                            Sign in
                        </ButtonLink>
                    }
                />
                <Route path="/login" element={<StateProbe />} />
            </Routes>,
        );

        // when
        await user.click(screen.getByRole("link", { name: "Sign in" }));

        // then
        expect(screen.getByText('Arrived with {"from":"/invite"}')).toBeInTheDocument();
    });

    it("renders its children as the accessible name of a link", () => {
        // given
        const label = "Start for free";

        // when
        renderWithProviders(<ButtonLink to="/register">{label}</ButtonLink>);

        // then
        expect(screen.getByRole("link", { name: label })).toBeInTheDocument();
    });

    it("points at the in-app route it is given", () => {
        // given
        const to = "/register";

        // when
        renderWithProviders(<ButtonLink to={to}>Start for free</ButtonLink>);

        // then
        expect(screen.getByRole("link", { name: "Start for free" })).toHaveAttribute("href", "/register");
    });

    it("navigates inside the app when an in-app link is followed", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(
            <Routes>
                <Route path="/" element={<ButtonLink to="/register">Start for free</ButtonLink>} />
                <Route path="/register" element={<h1>Register</h1>} />
            </Routes>,
        );

        // when
        await user.click(screen.getByRole("link", { name: "Start for free" }));

        // then
        expect(screen.getByRole("heading", { name: "Register" })).toBeInTheDocument();
    });

    it("keeps a plain href untouched for anchors and full page loads", () => {
        // given
        const href = "#pricing";

        // when
        renderWithProviders(<ButtonLink href={href}>See pricing</ButtonLink>);

        // then
        expect(screen.getByRole("link", { name: "See pricing" })).toHaveAttribute("href", "#pricing");
    });

    it("forwards native anchor attributes to the underlying element", () => {
        // given
        const title = "Opens the pricing section";

        // when
        renderWithProviders(
            <ButtonLink href="#pricing" title={title} aria-describedby="pricing-note">
                See pricing
            </ButtonLink>,
        );

        // then
        const link = screen.getByRole("link", { name: "See pricing" });
        expect(link).toHaveAttribute("title", title);
        expect(link).toHaveAttribute("aria-describedby", "pricing-note");
    });

    it("keeps a caller supplied class alongside its own classes", () => {
        // given
        const className = "pinned-to-bottom";

        // when
        renderWithProviders(
            <ButtonLink to="/register" className={className}>
                Start for free
            </ButtonLink>,
        );

        // then
        const link = screen.getByRole("link", { name: "Start for free" });
        expect(link).toHaveClass(className);
        expect(link.className.split(" ").length).toBeGreaterThan(1);
    });

    it("looks exactly like a button of the same variant and size", () => {
        // given
        const pairs = (
            <>
                <Button variant="primary" size="large">
                    button
                </Button>
                <ButtonLink to="/register" variant="primary" size="large">
                    to link
                </ButtonLink>
                <ButtonLink href="#pricing" variant="primary" size="large">
                    href link
                </ButtonLink>
            </>
        );

        // when
        renderWithProviders(pairs);

        // then
        const button = screen.getByRole("button", { name: "button" });
        expect(screen.getByRole("link", { name: "to link" }).className).toBe(button.className);
        expect(screen.getByRole("link", { name: "href link" }).className).toBe(button.className);
    });

    it("defaults to the secondary variant at the medium size, like a button", () => {
        // given
        const pair = (
            <>
                <Button>button</Button>
                <ButtonLink to="/">link</ButtonLink>
            </>
        );

        // when
        renderWithProviders(pair);

        // then
        expect(screen.getByRole("link", { name: "link" }).className).toBe(
            screen.getByRole("button", { name: "button" }).className,
        );
    });
});
