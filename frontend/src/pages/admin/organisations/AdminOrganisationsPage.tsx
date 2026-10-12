import { type SubmitEvent, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { DataTable } from "../../../components/DataTable/DataTable";
import { TextField } from "../../../components/FormField/TextField";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Pagination } from "../../../components/Pagination/Pagination";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { TextLink } from "../../../components/TextLink/TextLink";
import { PLAN_LABELS } from "../../../domain/entitlements";
import { useAdminOrganisations } from "../../../hooks/queries/adminOrganisations";
import type { AdminOrganisationResponse } from "../../../types/api";
import { formatDate } from "../../../utils/time";
import { AdminPage } from "../shared/AdminPage";
import styles from "./AdminOrganisationsPage.module.css";
import { UserName } from "./UserName";

const PAGE_SIZE = 20;

interface OrganisationOwnerProps {
    organisation: AdminOrganisationResponse;
}

function OrganisationOwner({ organisation }: OrganisationOwnerProps) {
    if (organisation.owner_id === undefined) {
        return <span className={styles.muted}>No owner</span>;
    }

    return (
        <span className={styles.stack}>
            <UserName userId={organisation.owner_id} name={organisation.owner_display_name ?? ""} />
            <bdi className={styles.muted}>{organisation.owner_email}</bdi>
        </span>
    );
}

export function AdminOrganisationsPage() {
    const [draft, setDraft] = useState("");
    const [search, setSearch] = useState("");
    const [offset, setOffset] = useState(0);
    const organisations = useAdminOrganisations({ search, limit: PAGE_SIZE, offset });

    function handleSearch(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();
        setSearch(draft.trim());
        setOffset(0);
    }

    const listing = organisations.data;

    return (
        <AdminPage
            title="Organisations"
            description="Every organisation on the site, with its owner, plan and size. Open one to see its teams and members."
        >
            <form className={styles.search} role="search" onSubmit={handleSearch}>
                <div className={styles.searchField}>
                    <TextField
                        label="Search organisations"
                        hint="Search by name or slug."
                        type="search"
                        value={draft}
                        onChange={e => setDraft(e.target.value)}
                    />
                </div>
                <div className={styles.searchAction}>
                    <Button type="submit" variant="primary">
                        Search
                    </Button>
                </div>
            </form>

            {organisations.error !== null && (
                <LoadFailure message={organisations.error.message} onRetry={organisations.refetch} />
            )}

            {listing === undefined && organisations.error === null && (
                <p className="loading">Loading organisations...</p>
            )}

            {listing !== undefined && listing.total === 0 && (
                <p className={styles.muted}>
                    {search === "" ? "No organisations yet." : "No organisations match that search."}
                </p>
            )}

            {listing !== undefined && listing.organisations.length > 0 && (
                <DataTable caption="Organisations">
                    <thead>
                        <tr>
                            <th scope="col">Name</th>
                            <th scope="col">Owner</th>
                            <th scope="col">Plan</th>
                            <th scope="col" className={styles.number}>
                                Members
                            </th>
                            <th scope="col" className={styles.number}>
                                Teams
                            </th>
                            <th scope="col" className={styles.number}>
                                Boards
                            </th>
                            <th scope="col">Created</th>
                            <th scope="col">Status</th>
                        </tr>
                    </thead>
                    <tbody>
                        {listing.organisations.map(organisation => (
                            <tr key={organisation.id}>
                                <td>
                                    <span className={styles.stack}>
                                        <TextLink to={`/admin/organisations/${encodeURIComponent(organisation.id)}`}>
                                            <bdi>{organisation.name}</bdi>
                                        </TextLink>
                                        <bdi className={styles.muted}>{organisation.slug}</bdi>
                                    </span>
                                </td>
                                <td>
                                    <OrganisationOwner organisation={organisation} />
                                </td>
                                <td>{PLAN_LABELS[organisation.plan]}</td>
                                <td className={styles.number}>{organisation.member_count}</td>
                                <td className={styles.number}>{organisation.team_count}</td>
                                <td className={styles.number}>{organisation.board_count}</td>
                                <td>{formatDate(organisation.created_at)}</td>
                                <td>
                                    {organisation.deleted_at === undefined ? (
                                        <StatusTag tone="success">Active</StatusTag>
                                    ) : (
                                        <StatusTag tone="danger">Deleted</StatusTag>
                                    )}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}

            {listing !== undefined && (
                <Pagination
                    label="Organisation pages"
                    offset={listing.offset}
                    limit={listing.limit}
                    total={listing.total}
                    onChange={setOffset}
                />
            )}
        </AdminPage>
    );
}
