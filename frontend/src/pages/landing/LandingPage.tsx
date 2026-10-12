import { ButtonLink } from "../../components/Button/ButtonLink";
import { MarketingLayout } from "../../components/marketing/MarketingLayout/MarketingLayout";
import { PricingPlans } from "../../components/marketing/PricingPlans/PricingPlans";
import { RevealBoard } from "../../components/marketing/RevealBoard/RevealBoard";
import { TextLink } from "../../components/TextLink/TextLink";
import { PRICING_PATH } from "../../domain/marketing";
import { usePageTitle } from "../../hooks/usePageTitle";
import styles from "./LandingPage.module.css";

const STEPS = [
    {
        title: "Create a board",
        body: "Pick a template or start from a blank board, then send the link to your team. Guests don't need an account.",
    },
    {
        title: "Add cards",
        body: "Everyone adds cards to the columns. Other people's cards stay hidden until the host reveals them.",
    },
    {
        title: "Vote",
        body: "Group similar cards together, then vote on the ones you want to discuss.",
    },
    {
        title: "Track actions",
        body: "Turn cards into action items with an owner and a due date. Open items carry over to your next retro.",
    },
];

const ALSO_FREE = [
    "Anonymous boards",
    "Countdown timer",
    "Password-protected boards",
    "See who is on the board",
    "Works on phones",
    "Keyboard accessible",
];

export function LandingPage() {
    usePageTitle();

    return (
        <MarketingLayout>
            <section className={styles.hero} aria-labelledby="landing-title">
                <div className={styles.heroInner}>
                    <div className={styles.pitch}>
                        <h1 id="landing-title" className={styles.title}>
                            Better sprint retrospectives for your team
                        </h1>
                        <p className={styles.lede}>
                            An online retro board where your team adds cards, votes on what matters and agrees on what
                            to do next. Free for one team, with unlimited boards.
                        </p>
                        <div className={styles.ctas}>
                            <ButtonLink to="/register" variant="primary" size="large">
                                Start for free
                            </ButtonLink>
                            <ButtonLink href="#pricing" size="large">
                                See pricing
                            </ButtonLink>
                        </div>
                    </div>
                    <RevealBoard />
                </div>
                <svg className={styles.wave} viewBox="0 0 1440 64" preserveAspectRatio="none" aria-hidden="true">
                    <path d="M0 40C180 8 360 8 540 32s360 32 540 8 270-28 360-16V64H0z" />
                </svg>
            </section>

            <section id="features" className={styles.section} aria-labelledby="features-title">
                <div className={styles.sectionHead}>
                    <h2 id="features-title" className={styles.heading}>
                        How it works
                    </h2>
                </div>
                <ol className={styles.steps}>
                    {STEPS.map((step, index) => (
                        <li key={step.title} className={styles.step}>
                            <span className={styles.stepNumber} aria-hidden="true">
                                {index + 1}
                            </span>
                            <h3 className={styles.stepTitle}>{step.title}</h3>
                            <p className={styles.stepBody}>{step.body}</p>
                        </li>
                    ))}
                </ol>
                <div className={styles.also}>
                    <h3 className={styles.alsoTitle}>Also on the free plan</h3>
                    <ul className={styles.alsoList}>
                        {ALSO_FREE.map(item => (
                            <li key={item}>{item}</li>
                        ))}
                    </ul>
                </div>
            </section>

            <section id="pricing" className={styles.section} aria-labelledby="pricing-title">
                <div className={styles.sectionHead}>
                    <h2 id="pricing-title" className={styles.heading}>
                        Pricing
                    </h2>
                    <p className={styles.intro}>Free for one team. Paid plans are charged per team, not per user.</p>
                </div>
                <PricingPlans />
                <p className={styles.more}>
                    <TextLink to={PRICING_PATH}>See the full pricing</TextLink>
                </p>
            </section>

            <section id="self-hosting" className={styles.section} aria-labelledby="self-hosting-title">
                <div className={styles.selfHosting}>
                    <div>
                        <h2 id="self-hosting-title" className={styles.heading}>
                            Host it yourself
                        </h2>
                        <p className={styles.intro}>
                            Prefer to keep your data on your own servers? A self-hosted version is on the way. It runs
                            as a single Docker image, for use within your own organisation.
                        </p>
                    </div>
                    <dl className={styles.facts}>
                        <div>
                            <dt>Requirements</dt>
                            <dd>Docker, Postgres and Valkey</dd>
                        </div>
                        <div>
                            <dt>Your data</dt>
                            <dd>Stays on your servers</dd>
                        </div>
                        <div>
                            <dt>Available</dt>
                            <dd>After launch</dd>
                        </div>
                    </dl>
                </div>
            </section>
        </MarketingLayout>
    );
}
