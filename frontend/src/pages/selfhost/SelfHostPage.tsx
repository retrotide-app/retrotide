import { ButtonLink } from "../../components/Button/ButtonLink";
import { CommandBlock } from "../../components/CommandBlock/CommandBlock";
import { DataTable } from "../../components/DataTable/DataTable";
import { MarketingHero } from "../../components/marketing/MarketingHero/MarketingHero";
import { MarketingLayout } from "../../components/marketing/MarketingLayout/MarketingLayout";
import { MarketingSection } from "../../components/marketing/MarketingSection/MarketingSection";
import { SELF_HOST_COMPOSE_PATH, SELF_HOST_PORT } from "../../domain/marketing";
import { usePageTitle } from "../../hooks/usePageTitle";
import { pageOrigin } from "../../platform/location";
import styles from "./SelfHostPage.module.css";

const SETTINGS = [
    {
        name: "port",
        meaning: `The port on your server that RetroTide answers on. It starts as ${SELF_HOST_PORT}; pick any port nothing else uses. Change only this setting, never the 4323 under ports.`,
    },
    {
        name: "site-address",
        meaning:
            "The address people type to reach RetroTide, such as https://retro.example.com. It must match exactly, or every page is refused. Without HTTPS it is http://your-server:your-port.",
    },
    {
        name: "site-mode",
        meaning: "private lets in only people you invite. public lets anyone create an account.",
    },
    {
        name: "database-password",
        meaning: "A long random password for RetroTide's database. You never type it anywhere else.",
    },
    {
        name: "database-admin-password",
        meaning: "A different long random password for the database's administrator.",
    },
];

export function SelfHostPage() {
    usePageTitle("Self-host RetroTide");

    const download = `mkdir retrotide && cd retrotide && curl -fsSLO ${pageOrigin()}${SELF_HOST_COMPOSE_PATH}`;

    return (
        <MarketingLayout>
            <MarketingHero
                title="Self-host RetroTide"
                eyebrow="Self-host"
                lede="Run RetroTide on your own server with Docker. One file to download, five settings to fill in, and your retrospectives stay on your own infrastructure."
            >
                <ButtonLink href={SELF_HOST_COMPOSE_PATH} variant="primary" size="large" download="">
                    Download docker-compose.yml
                </ButtonLink>
            </MarketingHero>

            <MarketingSection
                title="What you need"
                intro="A Linux server with Docker Engine and the Docker Compose plugin, and a domain name if you want HTTPS."
            />

            <MarketingSection title="Install">
                <ol className={styles.steps}>
                    <li>
                        <h3 className={styles.step}>Download the compose file</h3>
                        <p>On your server, make a folder for RetroTide and download the file into it.</p>
                        <CommandBlock command={download} label="Download the compose file" />
                    </li>
                    <li>
                        <h3 className={styles.step}>Fill in the settings</h3>
                        <p>Open docker-compose.yml. The settings are at the top, under x-settings. Fill in all five.</p>
                        <DataTable caption="The settings at the top of docker-compose.yml">
                            <thead>
                                <tr>
                                    <th scope="col">Setting</th>
                                    <th scope="col">What it is</th>
                                </tr>
                            </thead>
                            <tbody>
                                {SETTINGS.map(setting => (
                                    <tr key={setting.name}>
                                        <th scope="row">
                                            <code>{setting.name}</code>
                                        </th>
                                        <td>{setting.meaning}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </DataTable>
                    </li>
                    <li>
                        <h3 className={styles.step}>Start it</h3>
                        <CommandBlock command="docker compose up -d" label="Start RetroTide" />
                        <p>
                            RetroTide answers on your port within a minute, and sets up its database on the first start.
                        </p>
                    </li>
                    <li>
                        <h3 className={styles.step}>Create the first account</h3>
                        <p>
                            Open your site address in a browser and create an account. The first account becomes the
                            administrator. Set up email and Google or Microsoft sign-in from Admin, then Settings.
                        </p>
                    </li>
                </ol>
            </MarketingSection>

            <MarketingSection
                title="HTTPS"
                intro="Put a reverse proxy such as Caddy, nginx or Traefik in front of your port for your domain, and set site-address to the https address."
            />

            <MarketingSection title="Updating">
                <p>From the folder with docker-compose.yml, pull the latest release and restart.</p>
                <CommandBlock command="docker compose pull && docker compose up -d" label="Update RetroTide" />
            </MarketingSection>
        </MarketingLayout>
    );
}
