import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { StatusTag } from "./StatusTag";

describe("StatusTag", () => {
    it("shows its text", () => {
        // given
        const text = "Banned";

        // when
        renderWithProviders(<StatusTag tone="danger">{text}</StatusTag>);

        // then
        expect(screen.getByText(text)).toBeInTheDocument();
    });

    it("gives each tone its own look", () => {
        // given
        const tones = ["neutral", "success", "warning", "danger"] as const;

        // when
        renderWithProviders(
            <>
                {tones.map(tone => (
                    <StatusTag key={tone} tone={tone}>
                        {tone}
                    </StatusTag>
                ))}
            </>,
        );

        // then
        const looks = new Set(tones.map(tone => screen.getByText(tone).className));
        expect(looks.size).toBe(tones.length);
    });
});
