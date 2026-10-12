import { useId } from "react";
import { DailyChart } from "../../../components/DailyChart/DailyChart";
import { DataTable } from "../../../components/DataTable/DataTable";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Panel } from "../../../components/Panel/Panel";
import { type StatusTone, StatusTag } from "../../../components/StatusTag/StatusTag";
import { TextLink } from "../../../components/TextLink/TextLink";
import { ADMIN_HOME_PATH, visibleAdminSections } from "../../../domain/adminSections";
import type { StaffPermission } from "../../../domain/permissions";
import { useAdminStats } from "../../../hooks/queries/adminStats";
import { useAuth } from "../../../hooks/useAuth";
import { useSiteInfo } from "../../../hooks/useSiteInfo";
import type {
    AdminActivityCounts,
    AdminEmailStatus,
    AdminRecentSignup,
    AdminSiteHealth,
    AdminStatDay,
    AdminStatsResponse,
    AdminStatTotal,
} from "../../../types/api";
import { type ChartPoint, COUNT_FORMAT } from "../../../utils/chart";
import { formatExactDateTime, relativeTime } from "../../../utils/time";
import { AdminPage } from "../shared/AdminPage";
import styles from "./AdminDashboardPage.module.css";

interface TotalTile {
    key: keyof AdminStatsResponse["totals"];
    label: string;
}

interface HealthRow {
    label: string;
    value: string;
    tone: StatusTone;
}

interface ActivityRow {
    key: keyof AdminActivityCounts;
    label: string;
}

interface DashboardBodyProps {
    stats: AdminStatsResponse;
    permissions: readonly StaffPermission[];
}

const TOTAL_TILES: readonly TotalTile[] = [
    { key: "users", label: "Accounts" },
    { key: "verified_users", label: "Verified accounts" },
    { key: "organisations", label: "Organisations" },
    { key: "teams", label: "Teams" },
    { key: "boards", label: "Boards" },
    { key: "cards", label: "Cards" },
];

const ACTIVITY_ROWS: readonly ActivityRow[] = [
    { key: "users", label: "Accounts" },
    { key: "organisations", label: "Organisations" },
    { key: "boards", label: "Boards" },
    { key: "cards", label: "Cards" },
];

const EMAIL_HEALTH: Record<AdminEmailStatus, Omit<HealthRow, "label">> = {
    ready: { value: "Set up", tone: "success" },
    off: { value: "Not set up", tone: "warning" },
    failing: { value: "Set up but not working", tone: "danger" },
};

function seriesOf(days: readonly AdminStatDay[], key: "users" | "boards" | "cards"): ChartPoint[] {
    return days.map(day => ({ date: day.date, value: day[key] }));
}

function weekChange(total: AdminStatTotal): string {
    if (total.last_7_days === 0) {
        return "None new in the last 7 days";
    }

    return `${COUNT_FORMAT.format(total.last_7_days)} new in the last 7 days`;
}

function healthRows(health: AdminSiteHealth): HealthRow[] {
    return [
        {
            label: "Who can join",
            value: health.site_mode === "private" ? "Invitation only" : "Anyone",
            tone: "neutral",
        },
        {
            label: "Maintenance mode",
            value: health.maintenance_mode ? "On" : "Off",
            tone: health.maintenance_mode ? "warning" : "success",
        },
        { label: "Email sending", ...EMAIL_HEALTH[health.email_status] },
        {
            label: "Google sign-in",
            value: health.google_sign_in_configured ? "Set up" : "Not set up",
            tone: health.google_sign_in_configured ? "success" : "neutral",
        },
    ];
}

function Totals({ totals }: { totals: AdminStatsResponse["totals"] }) {
    const titleId = useId();

    return (
        <section aria-labelledby={titleId} className={styles.section}>
            <h3 id={titleId} className="visually-hidden">
                Totals
            </h3>
            <dl className={styles.tiles}>
                {TOTAL_TILES.map(tile => (
                    <div key={tile.key} className={styles.tile}>
                        <dt className={styles.tileLabel}>{tile.label}</dt>
                        <dd className={styles.tileValue}>{COUNT_FORMAT.format(totals[tile.key].total)}</dd>
                        <dd className={styles.tileChange}>{weekChange(totals[tile.key])}</dd>
                    </div>
                ))}
            </dl>
        </section>
    );
}

function Trends({ daily }: { daily: readonly AdminStatDay[] }) {
    const titleId = useId();

    return (
        <section aria-labelledby={titleId} className={styles.section}>
            <h3 id={titleId} className={styles.sectionTitle}>
                Last {daily.length} days
            </h3>
            <div className={styles.charts}>
                <div className={styles.chartWide}>
                    <DailyChart
                        title="New accounts"
                        headingLevel={4}
                        size="large"
                        points={seriesOf(daily, "users")}
                        noun={{ one: "account", other: "accounts" }}
                    />
                </div>
                <DailyChart
                    title="New boards"
                    headingLevel={4}
                    points={seriesOf(daily, "boards")}
                    noun={{ one: "board", other: "boards" }}
                />
                <DailyChart
                    title="New cards"
                    headingLevel={4}
                    points={seriesOf(daily, "cards")}
                    noun={{ one: "card", other: "cards" }}
                />
            </div>
        </section>
    );
}

function Activity({ activity }: { activity: AdminStatsResponse["activity"] }) {
    return (
        <Panel title="New in each period" headingLevel={3}>
            <DataTable caption="New accounts, organisations, boards and cards in each period">
                <thead>
                    <tr>
                        <th scope="col">Created</th>
                        <th scope="col" className={styles.number}>
                            24 hours
                        </th>
                        <th scope="col" className={styles.number}>
                            7 days
                        </th>
                        <th scope="col" className={styles.number}>
                            30 days
                        </th>
                    </tr>
                </thead>
                <tbody>
                    {ACTIVITY_ROWS.map(row => (
                        <tr key={row.key}>
                            <th scope="row">{row.label}</th>
                            <td className={styles.number}>{COUNT_FORMAT.format(activity.last_24_hours[row.key])}</td>
                            <td className={styles.number}>{COUNT_FORMAT.format(activity.last_7_days[row.key])}</td>
                            <td className={styles.number}>{COUNT_FORMAT.format(activity.last_30_days[row.key])}</td>
                        </tr>
                    ))}
                </tbody>
            </DataTable>
        </Panel>
    );
}

function Health({ health, canManageSettings }: { health: AdminSiteHealth; canManageSettings: boolean }) {
    return (
        <Panel title="Site health" headingLevel={3}>
            <dl className={styles.health}>
                {healthRows(health).map(row => (
                    <div key={row.label} className={styles.healthRow}>
                        <dt>{row.label}</dt>
                        <dd>
                            <StatusTag tone={row.tone}>{row.value}</StatusTag>
                        </dd>
                    </div>
                ))}
            </dl>
            {canManageSettings && <TextLink to="/admin/settings">Open settings</TextLink>}
        </Panel>
    );
}

function NewestAccounts({ signups }: { signups: readonly AdminRecentSignup[] }) {
    return (
        <Panel title="Newest accounts" headingLevel={3}>
            {signups.length === 0 && <p className={styles.muted}>No accounts yet.</p>}
            {signups.length > 0 && (
                <DataTable caption="Newest accounts, newest first">
                    <thead>
                        <tr>
                            <th scope="col">Name</th>
                            <th scope="col">Email</th>
                            <th scope="col">Status</th>
                            <th scope="col">Joined</th>
                        </tr>
                    </thead>
                    <tbody>
                        {signups.map(signup => (
                            <tr key={signup.id}>
                                <td>
                                    <TextLink to={`/admin/users/${encodeURIComponent(signup.id)}`}>
                                        <bdi>{signup.display_name}</bdi>
                                    </TextLink>
                                </td>
                                <td>
                                    <bdi>{signup.email}</bdi>
                                </td>
                                <td>
                                    {signup.email_verified ? (
                                        <StatusTag tone="success">Verified</StatusTag>
                                    ) : (
                                        <StatusTag tone="neutral">Unverified</StatusTag>
                                    )}
                                </td>
                                <td>
                                    <time dateTime={signup.created_at} title={formatExactDateTime(signup.created_at)}>
                                        {relativeTime(signup.created_at)}
                                    </time>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}
            <TextLink to="/admin/users">See every user</TextLink>
        </Panel>
    );
}

function DashboardBody({ stats, permissions }: DashboardBodyProps) {
    return (
        <>
            <p className={styles.asOf}>
                Figures as of {formatExactDateTime(stats.generated_at)}. Days run from midnight to midnight UTC.
            </p>
            <Totals totals={stats.totals} />
            <Trends daily={stats.daily} />
            <div className={styles.split}>
                <Activity activity={stats.activity} />
                <Health health={stats.health} canManageSettings={permissions.includes("manage_settings")} />
            </div>
            {stats.recent_signups !== undefined && <NewestAccounts signups={stats.recent_signups} />}
        </>
    );
}

function NoStatsAccess({ permissions }: { permissions: readonly StaffPermission[] }) {
    const { site_mode } = useSiteInfo();
    const sections = visibleAdminSections(permissions, site_mode).filter(section => section.path !== ADMIN_HOME_PATH);

    return (
        <div className={styles.noStats}>
            <p className={styles.muted}>Your role does not include the site stats.</p>
            {sections.length > 0 && (
                <nav aria-label="Admin pages you can open">
                    <ul className={styles.shortcuts}>
                        {sections.map(section => (
                            <li key={section.path}>
                                <TextLink to={section.path}>{section.label}</TextLink>
                            </li>
                        ))}
                    </ul>
                </nav>
            )}
        </div>
    );
}

export function AdminDashboardPage() {
    const { permissions } = useAuth();
    const canSeeStats = permissions.includes("view_stats");
    const stats = useAdminStats(canSeeStats);

    return (
        <AdminPage
            title="Dashboard"
            description="How the site is doing: new accounts, boards and cards, and site health."
        >
            {!canSeeStats && <NoStatsAccess permissions={permissions} />}
            {canSeeStats && stats.error !== null && (
                <LoadFailure message={stats.error.message} onRetry={stats.refetch} />
            )}
            {canSeeStats && stats.data === undefined && stats.error === null && (
                <p className="loading">Loading the site stats...</p>
            )}
            {canSeeStats && stats.data !== undefined && <DashboardBody stats={stats.data} permissions={permissions} />}
        </AdminPage>
    );
}
