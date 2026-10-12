import { ButtonLink } from "../../components/Button/ButtonLink";
import { DataTable } from "../../components/DataTable/DataTable";
import { InfoPanel } from "../../components/InfoPanel/InfoPanel";
import { MarketingHero } from "../../components/marketing/MarketingHero/MarketingHero";
import { MarketingLayout } from "../../components/marketing/MarketingLayout/MarketingLayout";
import { MarketingSection } from "../../components/marketing/MarketingSection/MarketingSection";
import { PricingPlans } from "../../components/marketing/PricingPlans/PricingPlans";
import {
    type BillingPeriod,
    EXTRA_TEAM_PRICE,
    FREE_DAILY_BOARD_LIMIT,
    FREE_MEMBER_LIMIT,
    formatPriceWithVat,
    ORGANISATION_INCLUDED_TEAMS,
    ORGANISATION_PRICE,
    TEAM_MEMBER_LIMIT,
    teamPlanCost,
} from "../../domain/pricing";
import { usePageTitle } from "../../hooks/usePageTitle";
import styles from "./PricingPage.module.css";

interface ComparisonRow {
    plan: string;
    cost: Readonly<Record<BillingPeriod, number>>;
}

const COMPARISON: readonly ComparisonRow[] = [
    {
        plan: `Team, ${ORGANISATION_INCLUDED_TEAMS} teams`,
        cost: {
            yearly: teamPlanCost(ORGANISATION_INCLUDED_TEAMS, "yearly"),
            monthly: teamPlanCost(ORGANISATION_INCLUDED_TEAMS, "monthly"),
        },
    },
    {
        plan: `Organisation, up to ${ORGANISATION_INCLUDED_TEAMS} teams`,
        cost: ORGANISATION_PRICE,
    },
];

const QUESTIONS = [
    {
        question: "Can I buy a paid plan today?",
        answer: "Not yet. Paid plans are coming. Until they arrive, every organisation is on Free, and the prices on this page are what we expect to charge.",
    },
    {
        question: "Do guests count as members?",
        answer: "No. Guests who join a board by link are free on every plan and never count towards a member limit.",
    },
    {
        question: "What counts as a member?",
        answer: `A member is a signed-in person who belongs to your organisation. Free allows ${FREE_MEMBER_LIMIT} members. Team allows ${TEAM_MEMBER_LIMIT} members in each team, and paid plans are charged per team, never per person.`,
    },
    {
        question: "Is there a limit on boards?",
        answer: `Boards are unlimited on every plan, with a fair-use limit of ${FREE_DAILY_BOARD_LIMIT} new boards a day on Free.`,
    },
    {
        question: "How is VAT shown?",
        answer: "Prices are shown excluding VAT for business customers. The UK price including VAT at 20% sits beside each one, because that is the total a UK consumer pays.",
    },
];

export function PricingPage() {
    usePageTitle("Pricing");

    return (
        <MarketingLayout>
            <MarketingHero
                title="Simple pricing, per team"
                eyebrow="Pricing"
                lede="Free for one team, with unlimited boards and guests. Paid plans will be charged per team, never per person, and they are coming soon."
            >
                <ButtonLink to="/register" variant="primary" size="large">
                    Start for free
                </ButtonLink>
            </MarketingHero>

            <MarketingSection
                title="Plans"
                intro="Prices exclude VAT for business customers. The UK price including 20% VAT is shown beside each one."
            >
                <PricingPlans />
            </MarketingSection>

            <MarketingSection
                title="Organisation compared with Team"
                intro={`Organisation always costs more than the same ${ORGANISATION_INCLUDED_TEAMS} teams on Team. You choose it for single sign-on, the audit log, policies and insights across teams, not as a volume discount.`}
            >
                <DataTable caption={`${ORGANISATION_INCLUDED_TEAMS} teams on Team compared with Organisation`}>
                    <thead>
                        <tr>
                            <th scope="col">Plan</th>
                            <th scope="col">A month, billed yearly</th>
                            <th scope="col">A month, billed monthly</th>
                        </tr>
                    </thead>
                    <tbody>
                        {COMPARISON.map(row => (
                            <tr key={row.plan}>
                                <th scope="row">{row.plan}</th>
                                <td>{formatPriceWithVat(row.cost.yearly)}</td>
                                <td>{formatPriceWithVat(row.cost.monthly)}</td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
                <p className={styles.footnote}>
                    {`Each team beyond ${ORGANISATION_INCLUDED_TEAMS} on Organisation costs ${formatPriceWithVat(EXTRA_TEAM_PRICE)} a month.`}
                </p>
            </MarketingSection>

            <MarketingSection title="Questions">
                <div className={styles.questions}>
                    {QUESTIONS.map(item => (
                        <InfoPanel key={item.question} title={item.question}>
                            <p>{item.answer}</p>
                        </InfoPanel>
                    ))}
                </div>
            </MarketingSection>
        </MarketingLayout>
    );
}
