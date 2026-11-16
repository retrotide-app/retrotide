import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { renderWithProviders } from "../../test-utils/render";
import { SelfHostPage } from "./SelfHostPage";

beforeEach(() => {
    document.title = "";
});

describe("SelfHostPage", () => {
    it("heads the page and titles the document", () => {
        // when
        renderWithProviders(<SelfHostPage />);

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Self-host RetroTide" })).toBeInTheDocument();
        expect(document.title).toContain("Self-host RetroTide");
    });

    it("offers the compose file as a download", () => {
        // when
        renderWithProviders(<SelfHostPage />);

        // then
        const download = screen.getByRole("link", { name: "Download docker-compose.yml" });
        expect(download).toHaveAttribute("href", "/self-host/docker-compose.yml");
        expect(download).toHaveAttribute("download");
    });

    it("downloads the compose file from this site, then starts it", () => {
        // when
        renderWithProviders(<SelfHostPage />);

        // then
        expect(screen.getByLabelText("Download the compose file")).toHaveTextContent(
            /^mkdir retrotide && cd retrotide && curl -fsSLO https?:\/\/\S+\/self-host\/docker-compose\.yml$/,
        );
        expect(screen.getByLabelText("Start RetroTide")).toHaveTextContent("docker compose up -d");
        expect(screen.getByLabelText("Update RetroTide")).toHaveTextContent(
            "docker compose pull && docker compose up -d",
        );
    });

    it("explains each of the five settings in the compose file", () => {
        // when
        renderWithProviders(<SelfHostPage />);

        // then
        const table = screen.getByRole("table", { name: "The settings at the top of docker-compose.yml" });
        const settings = Array.from(table.querySelectorAll("tbody th")).map(cell => cell.textContent);
        expect(settings).toEqual(["port", "site-address", "site-mode", "database-password", "database-admin-password"]);
    });
});
