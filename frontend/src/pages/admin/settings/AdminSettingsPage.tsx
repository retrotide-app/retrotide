import { type SubmitEvent, useState } from "react";
import { useSearchParams } from "react-router";
import { Button } from "../../../components/Button/Button";
import { ErrorBanner } from "../../../components/ErrorBanner/ErrorBanner";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Panel } from "../../../components/Panel/Panel";
import { SectionNav } from "../../../components/SectionNav/SectionNav";
import { Toast } from "../../../components/Toast/Toast";
import {
    changedSettings,
    EMAIL_GROUP_ID,
    isSettingShown,
    SETTINGS_SECTION_PARAM,
    SITE_SETTING_GROUPS,
    type SiteSettingGroup,
    type SiteSettingValues,
    settingGroupFor,
    settingValue,
} from "../../../domain/siteSettings";
import { useUpdateAdminSettings } from "../../../hooks/mutations/admin";
import { useAdminSettings } from "../../../hooks/queries/admin";
import { AdminPage } from "../shared/AdminPage";
import styles from "./AdminSettingsPage.module.css";
import { SettingControl } from "./SettingControl";
import { TestEmailControl } from "./TestEmailControl";

interface SectionFormProps {
    group: SiteSettingGroup;
    saved: SiteSettingValues;
}

const SECTION_ITEMS = SITE_SETTING_GROUPS.map(group => ({
    id: group.id,
    label: group.title,
    to: `?${SETTINGS_SECTION_PARAM}=${group.id}`,
}));

function SectionForm({ group, saved }: SectionFormProps) {
    const update = useUpdateAdminSettings();
    const [draft, setDraft] = useState<Record<string, string>>({});
    const [replacing, setReplacing] = useState<ReadonlySet<string>>(new Set());
    const changes = changedSettings(saved, draft);
    const unsaved = Object.keys(changes).length > 0;

    function change(key: string, value: string) {
        setDraft(previous => ({ ...previous, [key]: value }));
    }

    function startReplacing(key: string) {
        setReplacing(previous => new Set([...previous, key]));
        change(key, "");
    }

    function keepSaved(key: string) {
        setReplacing(previous => new Set([...previous].filter(held => held !== key)));
        setDraft(previous => Object.fromEntries(Object.entries(previous).filter(([held]) => held !== key)));
    }

    function save(next?: () => void) {
        update.mutate(
            { settings: changes },
            {
                onSuccess: () => {
                    setDraft({});
                    setReplacing(new Set());
                    next?.();
                },
            },
        );
    }

    function handleSubmit(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();
        save();
    }

    return (
        <form className={styles.form} onSubmit={handleSubmit} noValidate>
            <Panel title={group.title} headingLevel={3} description={group.description}>
                <div className={styles.fields}>
                    {group.fields
                        .filter(field => isSettingShown(field, saved, draft))
                        .map(field => (
                            <SettingControl
                                key={field.key}
                                field={field}
                                value={settingValue(saved, draft, field.key)}
                                savedValue={saved[field.key] ?? ""}
                                replacing={replacing.has(field.key)}
                                disabled={update.isPending}
                                onChange={value => change(field.key, value)}
                                onReplace={() => startReplacing(field.key)}
                                onKeep={() => keepSaved(field.key)}
                            />
                        ))}
                </div>
                <div className={styles.save}>
                    {update.error !== null && <ErrorBanner message={update.error.message} takeFocus />}
                    <div>
                        <Button type="submit" variant="primary" disabled={!unsaved || update.isPending}>
                            Save {group.title.toLowerCase()} settings
                        </Button>
                    </div>
                </div>
                {group.id === EMAIL_GROUP_ID && (
                    <TestEmailControl unsaved={unsaved} saving={update.isPending} saveThen={save} />
                )}
            </Panel>
            {update.isSuccess && (
                <Toast variant="success" onDismiss={update.reset}>
                    Settings saved.
                </Toast>
            )}
        </form>
    );
}

export function AdminSettingsPage() {
    const settings = useAdminSettings();
    const [params] = useSearchParams();
    const saved = settings.data?.settings;
    const group = settingGroupFor(params.get(SETTINGS_SECTION_PARAM));

    return (
        <AdminPage
            title="Settings"
            description="Settings for the whole site, one section at a time. Each section saves on its own."
        >
            {settings.error !== null && <LoadFailure message={settings.error.message} onRetry={settings.refetch} />}
            {saved === undefined && settings.error === null && <p className="loading">Loading settings...</p>}
            {saved !== undefined && (
                <div className={styles.layout}>
                    <SectionNav label="Settings sections" items={SECTION_ITEMS} current={group.id} />
                    <SectionForm key={group.id} group={group} saved={saved} />
                </div>
            )}
        </AdminPage>
    );
}
