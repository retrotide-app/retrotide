import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes, useLocation } from "react-router";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { TextLink } from "./TextLink";

function StateProbe() {
    const location = useLocation();

    return <p>{`Arrived with ${JSON.stringify(location.state)}`}</p>;
}

describe("TextLink", () => {
    it("hands the route state it is given to the page it opens", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(
            <Routes>
                <Route
                    path="/"
                    element={
                        <TextLink to="/login" state={{ from: "/o/harbour/teams" }}>
                            Sign in
                        </TextLink>
                    }
                />
                <Route path="/login" element={<StateProbe />} />
            </Routes>,
        );

        // when
        await user.click(screen.getByRole("link", { name: "Sign in" }));

        // then
        expect(screen.getByText('Arrived with {"from":"/o/harbour/teams"}')).toBeInTheDocument();
    });

    it("points at the in-app route it is given", () => {
        // given
        const to = "/privacy";

        // when
        renderWithProviders(<TextLink to={to}>Privacy notice</TextLink>);

        // then
        expect(screen.getByRole("link", { name: "Privacy notice" })).toHaveAttribute("href", "/privacy");
    });

    it("navigates inside the app when an in-app link is followed", async () => {
        // given
        const user = userEvent.setup();
        renderWithProviders(
            <Routes>
                <Route path="/" element={<TextLink to="/terms">Terms of service</TextLink>} />
                <Route path="/terms" element={<h1>Terms</h1>} />
            </Routes>,
        );

        // when
        await user.click(screen.getByRole("link", { name: "Terms of service" }));

        // then
        expect(screen.getByRole("heading", { name: "Terms" })).toBeInTheDocument();
    });

    it("keeps a plain href untouched for anchors and full page loads", () => {
        // given
        const href = "#features";

        // when
        renderWithProviders(<TextLink href={href}>Features</TextLink>);

        // then
        expect(screen.getByRole("link", { name: "Features" })).toHaveAttribute("href", "#features");
    });

    it("forwards native anchor attributes to the underlying element", () => {
        // given
        const title = "Read how we handle your data";

        // when
        renderWithProviders(
            <TextLink to="/privacy" title={title} aria-current="page">
                Privacy notice
            </TextLink>,
        );

        // then
        const link = screen.getByRole("link", { name: "Privacy notice" });
        expect(link).toHaveAttribute("title", title);
        expect(link).toHaveAttribute("aria-current", "page");
    });

    it("keeps a caller supplied class alongside its own classes", () => {
        // given
        const className = "footer-link";

        // when
        renderWithProviders(
            <TextLink to="/privacy" className={className}>
                Privacy notice
            </TextLink>,
        );

        // then
        const link = screen.getByRole("link", { name: "Privacy notice" });
        expect(link).toHaveClass(className);
        expect(link.className.split(" ").length).toBeGreaterThan(1);
    });

    it("defaults to the default tone", () => {
        // given
        const links = (
            <>
                <TextLink to="/">fallback</TextLink>
                <TextLink to="/" tone="default">
                    explicit
                </TextLink>
            </>
        );

        // when
        renderWithProviders(links);

        // then
        expect(screen.getByRole("link", { name: "fallback" }).className).toBe(
            screen.getByRole("link", { name: "explicit" }).className,
        );
    });

    it("gives every tone its own styling", () => {
        // given
        const tones = ["default", "muted", "inverse"] as const;

        // when
        renderWithProviders(
            <>
                {tones.map(tone => (
                    <TextLink key={tone} to="/" tone={tone}>
                        {tone}
                    </TextLink>
                ))}
            </>,
        );

        // then
        const seen = new Set<string>();
        for (const tone of tones) {
            seen.add(screen.getByRole("link", { name: tone }).className);
        }
        expect(seen.size).toBe(tones.length);
    });
});
