import { ButtonLink } from "../../components/Button/ButtonLink";
import { DataTable } from "../../components/DataTable/DataTable";
import { InfoPanel } from "../../components/InfoPanel/InfoPanel";
import { MarketingHero } from "../../components/marketing/MarketingHero/MarketingHero";
import { MarketingLayout } from "../../components/marketing/MarketingLayout/MarketingLayout";
import { MarketingSection } from "../../components/marketing/MarketingSection/MarketingSection";
import { TextLink } from "../../components/TextLink/TextLink";
import { PRICING_PATH } from "../../domain/marketing";
import { FREE_DAILY_BOARD_LIMIT, FREE_MEMBER_LIMIT } from "../../domain/pricing";
import { usePageTitle } from "../../hooks/usePageTitle";
import styles from "./CompareEasyRetroPage.module.css";

const CHECKED_ON = "2026-10";

const CHECKED_LABEL = "Checked in October 2026";

const ROWS = [
    {
        topic: "Boards on the free plan",
        retroTide: "Unlimited boards for one team",
        easyRetro: "1 public board a month",
    },
    {
        topic: "Unfinished actions carried to the next retro",
        retroTide: "Free on every plan",
        easyRetro: "A panel of unfinished team action items on paid plans, added in August 2026",
    },
    {
        topic: "AI template generator",
        retroTide: "No AI features, by choice",
        easyRetro: "Works on its Free plan",
    },
];

const DETAILS = [
    {
        title: "Boards on the free plan",
        body: `EasyRetro's free tier is 1 public board a month. RetroTide's free plan has unlimited boards for one team, with up to ${FREE_MEMBER_LIMIT} organisation members, unlimited guests and a fair-use limit of ${FREE_DAILY_BOARD_LIMIT} new boards a day.`,
    },
    {
        title: "Action carry-over is free",
        body: "On RetroTide, unfinished action items carry over to the team's next retro, and every new team board opens with a review of the previous actions. It is free, on every plan.",
    },
    {
        title: "No AI, by choice",
        body: "EasyRetro's AI template generator works on its Free plan. RetroTide has no AI features by choice: no summaries, no automatic grouping and no generated templates.",
    },
    {
        title: "Hidden cards stay hidden",
        body: "No RetroTide facilitator, owner included, can read hidden cards before the reveal. Until then the server sends nobody else's hidden card text to your browser, only a count per column.",
    },
];

export function CompareEasyRetroPage() {
    usePageTitle("RetroTide compared with EasyRetro");

    return (
        <MarketingLayout>
            <MarketingHero
                title="RetroTide compared with EasyRetro"
                eyebrow="Comparison"
                lede={
                    <>
                        A short, factual comparison. <time dateTime={CHECKED_ON}>{CHECKED_LABEL}</time>.
                    </>
                }
            >
                <ButtonLink to="/register" variant="primary" size="large">
                    Start for free
                </ButtonLink>
            </MarketingHero>

            <MarketingSection title="At a glance">
                <DataTable caption="RetroTide and EasyRetro side by side">
                    <thead>
                        <tr>
                            <th scope="col">Topic</th>
                            <th scope="col">RetroTide</th>
                            <th scope="col">EasyRetro</th>
                        </tr>
                    </thead>
                    <tbody>
                        {ROWS.map(row => (
                            <tr key={row.topic}>
                                <th scope="row">{row.topic}</th>
                                <td>{row.retroTide}</td>
                                <td>{row.easyRetro}</td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            </MarketingSection>

            <MarketingSection title="The details">
                <div className={styles.details}>
                    {DETAILS.map(detail => (
                        <InfoPanel key={detail.title} title={detail.title}>
                            <p>{detail.body}</p>
                        </InfoPanel>
                    ))}
                </div>
                <p className={styles.footnote}>
                    {`${CHECKED_LABEL}. Plans and features change, so check EasyRetro's own site for its current offer. `}
                    <TextLink to={PRICING_PATH}>See RetroTide's pricing</TextLink>
                </p>
            </MarketingSection>
        </MarketingLayout>
    );
}
