import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CardImage } from "./CardImage";

describe("CardImage", () => {
    it("shows the server-built image path with its description", () => {
        // when
        render(<CardImage path="/uploads/b1/m1.webp" alt="Image on the card Pair on reviews" />);

        // then
        const image = screen.getByRole("img", { name: "Image on the card Pair on reviews" });
        expect(image).toHaveAttribute("src", "/uploads/b1/m1.webp");
    });

    it("waits until the card scrolls near before loading the image", () => {
        // when
        render(<CardImage path="/uploads/b1/m1.webp" alt="Card image" />);

        // then
        expect(screen.getByRole("img", { name: "Card image" })).toHaveAttribute("loading", "lazy");
    });
});
