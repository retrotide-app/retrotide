import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { Banner, BannerButton, BannerDismiss, BannerLink, BannerNote, type BannerTone } from "./Banner";

describe("Banner", () => {
    it("shows its message alongside the actions it was given", () => {
        // given
        const actions = <BannerNote>Sent. Check your inbox.</BannerNote>;

        // when
        renderWithProviders(
            <Banner tone="info" actions={actions}>
                Verify your email to keep your boards.
            </Banner>,
        );

        // then
        expect(screen.getByText("Verify your email to keep your boards.")).toBeInTheDocument();
        expect(screen.getByText("Sent. Check your inbox.")).toBeInTheDocument();
    });

    it("announces itself with the role and label it was given", () => {
        // given
        const label = "Scheduled maintenance";

        // when
        renderWithProviders(
            <Banner tone="warning" role="region" label={label}>
                RetroTide will be read only for ten minutes at 22:00.
            </Banner>,
        );

        // then
        expect(screen.getByRole("region", { name: label })).toHaveTextContent(
            "RetroTide will be read only for ten minutes at 22:00.",
        );
    });

    it("gives each tone a look of its own", () => {
        // given
        const tones: BannerTone[] = ["danger", "warning", "info", "success"];

        // when
        renderWithProviders(
            <>
                {tones.map(tone => (
                    <Banner key={tone} tone={tone} role="status" label={tone}>
                        {`A ${tone} message`}
                    </Banner>
                ))}
            </>,
        );

        // then
        const classNames = tones.map(tone => screen.getByRole("status", { name: tone }).className);
        expect(new Set(classNames).size).toBe(tones.length);
    });
});

describe("BannerButton", () => {
    it("runs its action when pressed", async () => {
        // given
        const onClick = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<BannerButton onClick={onClick}>Reload now</BannerButton>);

        // when
        await user.click(screen.getByRole("button", { name: "Reload now" }));

        // then
        expect(onClick).toHaveBeenCalledOnce();
    });

    it("cannot be pressed while it is disabled", () => {
        // given
        const onClick = vi.fn();

        // when
        renderWithProviders(
            <BannerButton onClick={onClick} disabled>
                Sending...
            </BannerButton>,
        );

        // then
        expect(screen.getByRole("button", { name: "Sending..." })).toBeDisabled();
    });
});

describe("BannerLink", () => {
    it("links to the page it was given", () => {
        // given
        const to = "/settings/email";

        // when
        renderWithProviders(<BannerLink to={to}>Add email</BannerLink>);

        // then
        expect(screen.getByRole("link", { name: "Add email" })).toHaveAttribute("href", to);
    });
});

describe("BannerDismiss", () => {
    it("dismisses the banner when pressed", async () => {
        // given
        const onDismiss = vi.fn();
        const user = userEvent.setup();
        renderWithProviders(<BannerDismiss onDismiss={onDismiss} />);

        // when
        await user.click(screen.getByRole("button", { name: "Dismiss" }));

        // then
        expect(onDismiss).toHaveBeenCalledOnce();
    });
});
