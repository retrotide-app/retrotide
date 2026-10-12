import { type SubmitEvent, useState } from "react";
import { Button } from "../../../components/Button/Button";
import { DataTable } from "../../../components/DataTable/DataTable";
import { TextField } from "../../../components/FormField/TextField";
import { LoadFailure } from "../../../components/LoadFailure/LoadFailure";
import { Pagination } from "../../../components/Pagination/Pagination";
import { StatusTag } from "../../../components/StatusTag/StatusTag";
import { TextLink } from "../../../components/TextLink/TextLink";
import { staffRoleLabel } from "../../../domain/staffRoles";
import { useAdminUsers } from "../../../hooks/queries/admin";
import type { AdminUserResponse } from "../../../types/api";
import { formatDate } from "../../../utils/time";
import { AdminPage } from "../shared/AdminPage";
import styles from "./AdminUsersPage.module.css";

const PAGE_SIZE = 20;

interface UserStatusProps {
    user: AdminUserResponse;
}

function UserStatus({ user }: UserStatusProps) {
    const restricted = user.banned || user.locked || !user.email_verified;

    return (
        <span className={styles.statuses}>
            {user.banned && <StatusTag tone="danger">Banned</StatusTag>}
            {user.locked && <StatusTag tone="warning">Locked</StatusTag>}
            {!user.email_verified && <StatusTag tone="neutral">Unverified</StatusTag>}
            {!restricted && <StatusTag tone="success">Active</StatusTag>}
        </span>
    );
}

export function AdminUsersPage() {
    const [draft, setDraft] = useState("");
    const [search, setSearch] = useState("");
    const [offset, setOffset] = useState(0);
    const users = useAdminUsers({ search, limit: PAGE_SIZE, offset });

    function handleSearch(e: SubmitEvent<HTMLFormElement>) {
        e.preventDefault();
        setSearch(draft.trim());
        setOffset(0);
    }

    const listing = users.data;

    return (
        <AdminPage
            title="Users"
            description="Every account on the site. Open one to change its role, email address or access."
        >
            <form className={styles.search} role="search" onSubmit={handleSearch}>
                <div className={styles.searchField}>
                    <TextField
                        label="Search users"
                        hint="Search by name or email address."
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

            {users.error !== null && <LoadFailure message={users.error.message} onRetry={users.refetch} />}

            {listing === undefined && users.error === null && <p className="loading">Loading users...</p>}

            {listing !== undefined && listing.total === 0 && (
                <p className={styles.empty}>{search === "" ? "No users yet." : "No users match that search."}</p>
            )}

            {listing !== undefined && listing.users.length > 0 && (
                <DataTable caption="Users">
                    <thead>
                        <tr>
                            <th scope="col">Name</th>
                            <th scope="col">Email</th>
                            <th scope="col">Role</th>
                            <th scope="col">Status</th>
                            <th scope="col">Joined</th>
                        </tr>
                    </thead>
                    <tbody>
                        {listing.users.map(user => (
                            <tr key={user.id}>
                                <td>
                                    <TextLink to={`/admin/users/${encodeURIComponent(user.id)}`}>
                                        <bdi>{user.display_name}</bdi>
                                    </TextLink>
                                </td>
                                <td>
                                    <bdi>{user.email}</bdi>
                                </td>
                                <td>{staffRoleLabel(user.role)}</td>
                                <td>
                                    <UserStatus user={user} />
                                </td>
                                <td>{formatDate(user.created_at)}</td>
                            </tr>
                        ))}
                    </tbody>
                </DataTable>
            )}

            {listing !== undefined && (
                <Pagination
                    label="User pages"
                    offset={listing.offset}
                    limit={listing.limit}
                    total={listing.total}
                    onChange={setOffset}
                />
            )}
        </AdminPage>
    );
}
