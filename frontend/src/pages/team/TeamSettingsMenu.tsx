import { useState } from "react";
import { useNavigate } from "react-router";
import { ConfirmDialog } from "../../components/ConfirmDialog/ConfirmDialog";
import { Menu, type MenuItem } from "../../components/Menu/Menu";
import { Toast } from "../../components/Toast/Toast";
import { BOARD_NOUN } from "../../domain/board/organisationBoards";
import { isArchived, organisationPath } from "../../domain/organisations";
import { useArchiveTeam, useDeleteTeam, useRenameTeam } from "../../hooks/mutations/teams";
import { useTeamBoards } from "../../hooks/queries/boards";
import { countOf } from "../../utils/chart";
import type { OrganisationResponse, TeamResponse } from "../../types/api";
import { ActionFailure } from "../org/shared/ActionFailure";
import { TeamNameDialog } from "../org/shared/TeamNameDialog";
import pageStyles from "./TeamPage.module.css";

interface TeamSettingsMenuProps {
    organisation: OrganisationResponse;
    team: TeamResponse;
    renames: boolean;
    manages: boolean;
}

type Dialog = "rename" | "archive" | "delete" | null;

export function TeamSettingsMenu({ organisation, team, renames, manages }: TeamSettingsMenuProps) {
    const navigate = useNavigate();
    const rename = useRenameTeam();
    const archive = useArchiveTeam();
    const deletion = useDeleteTeam();
    const boards = useTeamBoards(team.id);
    const [dialog, setDialog] = useState<Dialog>(null);

    const archived = isArchived(team);
    const target = { orgId: organisation.id, teamId: team.id };

    const items: MenuItem[] = [];

    if (renames && !archived) {
        items.push({
            label: "Rename",
            onSelect: () => {
                rename.reset();
                setDialog("rename");
            },
        });
    }

    if (manages && !archived) {
        items.push({
            label: "Archive",
            onSelect: () => {
                archive.reset();
                setDialog("archive");
            },
        });
    }

    if (manages) {
        items.push({
            label: "Delete",
            tone: "danger",
            onSelect: () => {
                deletion.reset();
                setDialog("delete");
            },
        });
    }

    function handleRename(name: string) {
        rename.mutate({ ...target, name }, { onSuccess: () => setDialog(null) });
    }

    function handleArchive() {
        archive.mutate(target, { onSuccess: () => setDialog(null) });
    }

    function handleDelete() {
        deletion.mutate(target, {
            onSuccess: () => {
                navigate(organisationPath(organisation.slug, "teams"), { replace: true });
            },
        });
    }

    const archiveBody = (
        <div className={pageStyles.dialogBody}>
            <p>
                <bdi>{team.name}</bdi> becomes read-only: nobody can rename it, change its members or add boards to it.
                You cannot undo this.
            </p>
            {archive.error !== null && <ActionFailure error={archive.error} takeFocus />}
        </div>
    );

    const boardCount = boards.data?.boards.length ?? 0;

    const deleteBody = (
        <div className={pageStyles.dialogBody}>
            <p>
                {boardCount > 0 ? (
                    <>
                        Deleting <bdi>{team.name}</bdi> also deletes its {countOf(boardCount, BOARD_NOUN)} and every
                        card on {boardCount === 1 ? "it" : "them"}.
                    </>
                ) : (
                    <>
                        <bdi>{team.name}</bdi> and its member list are deleted.
                    </>
                )}{" "}
                You cannot undo this.
            </p>
            {deletion.error !== null && <ActionFailure error={deletion.error} takeFocus />}
        </div>
    );

    return (
        <>
            <Menu label="Team settings" items={items} />

            <TeamNameDialog
                open={dialog === "rename"}
                title={`Rename ${team.name}`}
                submitLabel="Save name"
                initialName={team.name}
                busy={rename.isPending}
                error={rename.error}
                onSubmit={handleRename}
                onClose={() => setDialog(null)}
            />

            <ConfirmDialog
                open={dialog === "archive"}
                title={`Archive ${team.name}?`}
                body={archiveBody}
                confirmLabel="Archive team"
                destructive
                busy={archive.isPending}
                onConfirm={handleArchive}
                onCancel={() => setDialog(null)}
            />

            <ConfirmDialog
                open={dialog === "delete"}
                title={`Delete ${team.name}?`}
                body={deleteBody}
                confirmLabel="Delete team"
                destructive
                busy={deletion.isPending}
                onConfirm={handleDelete}
                onCancel={() => setDialog(null)}
            />

            {rename.isSuccess && (
                <Toast variant="success" onDismiss={rename.reset}>
                    Team renamed.
                </Toast>
            )}

            {archive.isSuccess && (
                <Toast variant="success" onDismiss={archive.reset}>
                    <bdi>{team.name}</bdi> archived.
                </Toast>
            )}
        </>
    );
}
