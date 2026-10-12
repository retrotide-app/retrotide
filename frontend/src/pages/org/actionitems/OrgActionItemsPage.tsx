import { useId, useState } from "react";
import { MentionText } from "../../../components/board/mentions/MentionText";
import { Button } from "../../../components/Button/Button";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Menu, type MenuItem } from "../../../components/Menu/Menu";
import { Pagination } from "../../../components/Pagination/Pagination";
import { Select } from "../../../components/Select/Select";
import { StatusTag, type StatusTone } from "../../../components/StatusTag/StatusTag";
import { TextLink } from "../../../components/TextLink/TextLink";
import { Toast } from "../../../components/Toast/Toast";
import { ToggleGroup } from "../../../components/ToggleGroup/ToggleGroup";
import {
    ACTION_ITEM_STATUS_LABELS,
    type AssigneeFilter,
    assigneeChoices,
    assigneeQuery,
    completionLabel,
    formatDueDate,
    nextStatus,
    pickedPerson,
    type StatusFilter,
    statusFilterOptions,
    statusQuery,
    type TeamFilter,
    teamChoices,
    teamQuery,
} from "../../../domain/board/actionItems";
import { boardPath } from "../../../domain/board/paths";
import { isDeleted } from "../../../domain/organisations";
import { type ListedActionItemMark, useMarkListedActionItem } from "../../../hooks/mutations/actionItems";
import { useOrganisationActionItems } from "../../../hooks/queries/actionItems";
import { useTeams } from "../../../hooks/queries/teams";
import { useCurrentOrg } from "../../../hooks/useCurrentOrg";
import type { ActionItemListItem, ActionItemStatus, OrganisationResponse, PersonView } from "../../../types/api";
import { DeletedOrganisation } from "../shared/DeletedOrganisation";
import { OrgPage } from "../shared/OrgPage";
import styles from "./OrgActionItemsPage.module.css";

const PAGE_TITLE = "Action items";

const PAGE_SIZE = 50;

const STATUS_TONES: Readonly<Record<ActionItemStatus, StatusTone>> = {
    open: "warning",
    done: "success",
    dropped: "neutral",
};

interface ActionItemsHomeProps {
    organisation: OrganisationResponse;
}

interface ListedItemProps {
    organisation: OrganisationResponse;
    item: ActionItemListItem;
    busy: boolean;
    onMark: (mark: ListedActionItemMark) => void;
}

function ListedItem({ organisation, item, busy, onMark }: ListedItemProps) {
    const titleId = useId();
    const due = formatDueDate(item.due_on);
    const { board } = item;
    const toggled = nextStatus(item.status);
    const menu: MenuItem[] = [];

    if (board !== null && item.status !== "dropped") {
        menu.push({
            label: "Drop",
            onSelect: () => onMark({ boardId: board.id, itemId: item.id, status: "dropped" }),
        });
    }

    return (
        <li className={styles.item} aria-labelledby={titleId}>
            <div className={styles.main}>
                <span className={styles.title}>
                    <MentionText id={titleId} text={item.title} mentions={item.mentions} />
                </span>
                <span className={styles.meta}>
                    {item.assignee === null ? "No one assigned" : <bdi>{item.assignee.display_name}</bdi>}
                    {due !== "" && ` · Due ${due}`}
                    {" · "}
                    {board === null ? (
                        "Board deleted"
                    ) : (
                        <TextLink to={boardPath(organisation.slug, board.id)}>
                            <bdi>{board.title}</bdi>
                        </TextLink>
                    )}
                    {item.team_name !== undefined && (
                        <>
                            {" · "}
                            <bdi>{item.team_name}</bdi>
                        </>
                    )}
                </span>
            </div>
            <div className={styles.side}>
                <StatusTag tone={STATUS_TONES[item.status]}>{ACTION_ITEM_STATUS_LABELS[item.status]}</StatusTag>
                {board !== null && (
                    <Button
                        type="button"
                        size="small"
                        disabled={busy}
                        onClick={() => onMark({ boardId: board.id, itemId: item.id, status: toggled })}
                    >
                        {toggled === "done" ? "Mark done" : "Reopen"}
                    </Button>
                )}
                {menu.length > 0 && <Menu label="Action item options" items={menu} iconOnly size="small" />}
            </div>
        </li>
    );
}

function ActionItemsHome({ organisation }: ActionItemsHomeProps) {
    const [status, setStatus] = useState<StatusFilter>("open");
    const [assignee, setAssignee] = useState<AssigneeFilter>("anyone");
    const [picked, setPicked] = useState<PersonView | null>(null);
    const [team, setTeam] = useState<TeamFilter>("all");
    const [offset, setOffset] = useState(0);

    const query = {
        status: statusQuery(status),
        assignee: assigneeQuery(assignee),
        team: teamQuery(team),
        limit: PAGE_SIZE,
        offset,
    };
    const items = useOrganisationActionItems(organisation.id, query);
    const teams = useTeams(organisation.id);
    const mark = useMarkListedActionItem(organisation.id);

    const listing = items.data;
    const listed = listing?.action_items ?? [];
    const filtered = status !== "open" || assignee !== "anyone" || team !== "all";

    function chooseAssignee(value: string) {
        setPicked(pickedPerson(listed, value) ?? picked);
        setAssignee(value);
        setOffset(0);
    }

    const summary = listing !== undefined && (
        <div className={styles.summary}>
            <StatusTag tone={listing.completion_rate === 1 ? "success" : "neutral"}>
                {completionLabel(listing.completion_rate)}
            </StatusTag>
            <StatusTag tone="neutral">{`${listing.counts.done} done, ${listing.counts.open} open`}</StatusTag>
        </div>
    );

    return (
        <OrgPage
            title={PAGE_TITLE}
            description="Every action item from the retros you can open in this organisation, and how many are done."
            status={summary}
        >
            <div className={styles.toolbar}>
                <ToggleGroup
                    label="Status"
                    options={statusFilterOptions(listing?.counts)}
                    value={status}
                    onChange={next => {
                        setStatus(next);
                        setOffset(0);
                    }}
                />
                <div className={styles.selects}>
                    <Select
                        size="small"
                        aria-label="Assignee"
                        className={styles.select}
                        value={assignee}
                        onChange={e => chooseAssignee(e.target.value)}
                    >
                        {assigneeChoices(listed, picked).map(choice => (
                            <option key={choice.value} value={choice.value}>
                                {choice.label}
                            </option>
                        ))}
                    </Select>
                    <Select
                        size="small"
                        aria-label="Boards"
                        className={styles.select}
                        value={team}
                        onChange={e => {
                            setTeam(e.target.value);
                            setOffset(0);
                        }}
                    >
                        {teamChoices(teams.data?.teams ?? []).map(choice => (
                            <option key={choice.value} value={choice.value}>
                                {choice.label}
                            </option>
                        ))}
                    </Select>
                </div>
            </div>

            {items.error !== null && <LoadFailure message={items.error.message} onRetry={items.refetch} />}
            {teams.error !== null && <LoadFailure message={teams.error.message} onRetry={teams.refetch} />}
            {listing === undefined && items.error === null && <p className="loading">Loading action items...</p>}

            {listing !== undefined && listed.length === 0 && (
                <p className={styles.hint}>
                    {filtered
                        ? "No action items match these filters."
                        : "No open action items. They are added in the Actions stage of a retro."}
                </p>
            )}

            {listed.length > 0 && (
                <ul className={styles.list} aria-label="Action items">
                    {listed.map(item => (
                        <ListedItem
                            key={item.id}
                            organisation={organisation}
                            item={item}
                            busy={mark.isPending}
                            onMark={change => {
                                mark.reset();
                                mark.mutate(change);
                            }}
                        />
                    ))}
                </ul>
            )}

            {listing !== undefined && (
                <Pagination
                    label="Action item pages"
                    offset={listing.offset}
                    limit={listing.limit}
                    total={listing.total}
                    onChange={setOffset}
                />
            )}

            {mark.error !== null && (
                <Toast variant="error" onDismiss={mark.reset}>
                    {mark.error.message}
                </Toast>
            )}
        </OrgPage>
    );
}

export function OrgActionItemsPage() {
    const { current } = useCurrentOrg();

    if (current === null) {
        return null;
    }

    if (isDeleted(current)) {
        return <DeletedOrganisation organisation={current} pageTitle={PAGE_TITLE} />;
    }

    return <ActionItemsHome key={current.id} organisation={current} />;
}
