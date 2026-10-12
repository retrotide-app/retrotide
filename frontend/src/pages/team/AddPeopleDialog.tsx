import { type ReactNode, useState } from "react";
import { Banner } from "../../components/Banner/Banner";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { FieldRow } from "../../components/FieldRow/FieldRow";
import { FormDialog } from "../../components/FormDialog/FormDialog";
import { FormField } from "../../components/FormField/FormField";
import { TextField } from "../../components/FormField/TextField";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { Modal } from "../../components/Modal/Modal";
import { Select } from "../../components/Select/Select";
import { ToggleGroup, type ToggleOption } from "../../components/ToggleGroup/ToggleGroup";
import {
    invitableRoles,
    TEAM_ROLE_LABELS,
    TEAM_ROLES,
    teamMemberAdditionRoles,
    type TeamViewer,
} from "../../domain/organisationRoles";
import { useCreateInvite } from "../../hooks/mutations/invites";
import { useAddTeamMember } from "../../hooks/mutations/teams";
import { useOrganisationMembers } from "../../hooks/queries/organisations";
import { useCopyText } from "../../hooks/useCopyText";
import { useSiteInfo } from "../../hooks/useSiteInfo";
import type {
    OrganisationMemberResponse,
    OrganisationResponse,
    TeamMemberResponse,
    TeamResponse,
    TeamRole,
} from "../../types/api";
import { errorCode, errorStatus } from "../../utils/errorMessage";
import { ActionFailure } from "../org/shared/ActionFailure";
import styles from "./AddPeopleDialog.module.css";

interface AddPeopleDialogProps {
    open: boolean;
    organisation: OrganisationResponse;
    team: TeamResponse;
    viewer: TeamViewer;
    members: readonly TeamMemberResponse[];
    onDone: (notice: ReactNode) => void;
    onClose: () => void;
}

type AddPeopleProps = Omit<AddPeopleDialogProps, "open">;

interface Candidate {
    person: OrganisationMemberResponse;
    roles: TeamRole[];
}

type Mode = "existing" | "invite";

const PERSON_MISSING = "Choose who to add.";

const EMAIL_MISSING = "Enter an email address.";

const LINK_MISSING = "The invite was created, but the answer did not include its link.";

const DEFAULT_ROLE: TeamRole = "member";

const ADDRESS_REFUSAL_CODES: ReadonlySet<string> = new Set(["already_member", "invite_pending"]);

function candidatesFor(
    viewer: TeamViewer,
    people: readonly OrganisationMemberResponse[],
    members: readonly TeamMemberResponse[],
): Candidate[] {
    const onTeam = new Set(members.map(member => member.user_id));

    return people
        .filter(person => !onTeam.has(person.user_id))
        .map(person => ({
            person,
            roles: teamMemberAdditionRoles(viewer, {
                userId: person.user_id,
                orgRole: person.role,
                teamRole: null,
                siteRole: person.site_role ?? null,
            }),
        }))
        .filter(candidate => candidate.roles.length > 0);
}

function pickRole(offered: readonly TeamRole[], wanted: TeamRole): TeamRole {
    if (offered.includes(wanted)) {
        return wanted;
    }

    return offered.includes(DEFAULT_ROLE) ? DEFAULT_ROLE : (offered[0] ?? DEFAULT_ROLE);
}

function modeFor(addsExisting: boolean, invites: boolean, chosen: Mode): Mode {
    if (!addsExisting) {
        return "invite";
    }

    if (!invites) {
        return "existing";
    }

    return chosen;
}

function addressRefusal(error: Error | null): string | null {
    if (error === null) {
        return null;
    }

    if (errorStatus(error) === 400 || ADDRESS_REFUSAL_CODES.has(errorCode(error) ?? "")) {
        return error.message;
    }

    return null;
}

function dialogTitle(team: TeamResponse): string {
    return `Add people to ${team.name}`;
}

interface ExistingMemberFormProps {
    team: TeamResponse;
    candidates: readonly Candidate[];
    intro: ReactNode;
    onDone: (notice: ReactNode) => void;
    onClose: () => void;
}

function ExistingMemberForm({ team, candidates, intro, onDone, onClose }: ExistingMemberFormProps) {
    const addition = useAddTeamMember();
    const [chosenId, setChosenId] = useState("");
    const [wantedRole, setWantedRole] = useState<TeamRole>(DEFAULT_ROLE);
    const [problem, setProblem] = useState<string | null>(null);

    const chosen = candidates.find(candidate => candidate.person.user_id === chosenId) ?? null;
    const offered =
        chosen?.roles ?? TEAM_ROLES.filter(role => candidates.some(candidate => candidate.roles.includes(role)));
    const role = pickRole(offered, wantedRole);

    function handleSubmit() {
        if (chosen === null) {
            addition.reset();
            setProblem(PERSON_MISSING);
            return;
        }

        const name = chosen.person.display_name;

        setProblem(null);
        addition.mutate(
            { teamId: team.id, user_id: chosen.person.user_id, role },
            {
                onSuccess: () => {
                    onDone(
                        <>
                            <bdi>{name}</bdi> added to <bdi>{team.name}</bdi>.
                        </>,
                    );
                },
            },
        );
    }

    return (
        <FormDialog
            open
            title={dialogTitle(team)}
            submitLabel="Add to team"
            busy={addition.isPending}
            onSubmit={handleSubmit}
            onCancel={onClose}
        >
            {intro}
            {problem !== null && <ErrorBanner message={problem} takeFocus />}
            {problem === null && addition.error !== null && <ActionFailure error={addition.error} takeFocus />}
            <FieldRow>
                <FormField label="Person" error={problem}>
                    {control => (
                        <Select {...control} value={chosenId} onChange={e => setChosenId(e.target.value)}>
                            <option value="">Choose a person</option>
                            {candidates.map(({ person }) => (
                                <option key={person.user_id} value={person.user_id}>
                                    {`${person.display_name} (${person.email})`}
                                </option>
                            ))}
                        </Select>
                    )}
                </FormField>
                <FormField label="Team role">
                    {control => (
                        <Select
                            {...control}
                            value={role}
                            onChange={e => {
                                const picked = offered.find(option => option === e.target.value);

                                if (picked !== undefined) {
                                    setWantedRole(picked);
                                }
                            }}
                        >
                            {offered.map(option => (
                                <option key={option} value={option}>
                                    {TEAM_ROLE_LABELS[option]}
                                </option>
                            ))}
                        </Select>
                    )}
                </FormField>
            </FieldRow>
        </FormDialog>
    );
}

interface InviteFormProps {
    organisation: OrganisationResponse;
    team: TeamResponse;
    intro: ReactNode;
    onDone: (notice: ReactNode) => void;
    onClose: () => void;
}

function InviteForm({ organisation, team, intro, onDone, onClose }: InviteFormProps) {
    const emailInvite = useCreateInvite();
    const linkInvite = useCreateInvite();
    const copy = useCopyText();
    const [email, setEmail] = useState("");
    const [problem, setProblem] = useState<string | null>(null);

    const fieldError = problem ?? addressRefusal(emailInvite.error);
    const created = linkInvite.isSuccess ? linkInvite.data : undefined;
    const url = created?.url;
    const invite = { orgId: organisation.id, role: "member" as const, team_id: team.id };

    function handleSubmit() {
        const address = email.trim();

        if (address === "") {
            emailInvite.reset();
            setProblem(EMAIL_MISSING);
            return;
        }

        setProblem(null);
        linkInvite.reset();
        emailInvite.mutate(
            { ...invite, email: address },
            {
                onSuccess: () => {
                    onDone(
                        <>
                            Invite sent to <bdi>{address}</bdi>.
                        </>,
                    );
                },
            },
        );
    }

    function createLink() {
        setProblem(null);
        emailInvite.reset();
        copy.reset();
        linkInvite.mutate(invite);
    }

    const linkAction = (
        <Button type="button" onClick={createLink} disabled={linkInvite.isPending}>
            Create a link instead
        </Button>
    );

    return (
        <FormDialog
            open
            title={dialogTitle(team)}
            submitLabel="Send invite"
            busy={emailInvite.isPending}
            extraActions={linkAction}
            onSubmit={handleSubmit}
            onCancel={onClose}
        >
            {intro}
            <p className={styles.note}>
                They join <bdi>{organisation.name}</bdi> as a member and go straight onto this team.
            </p>
            {problem !== null && <ErrorBanner message={problem} takeFocus />}
            {problem === null && emailInvite.error !== null && <ActionFailure error={emailInvite.error} takeFocus />}
            {linkInvite.error !== null && <ActionFailure error={linkInvite.error} takeFocus />}
            <TextField
                label="Email address"
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                error={fieldError}
                autoComplete="off"
                required
            />
            {created !== undefined && url === undefined && <ErrorBanner message={LINK_MISSING} takeFocus />}
            {url !== undefined && (
                <div className={styles.created}>
                    <Banner tone="success" role="status">
                        Invite link created. Copy it now, because it is shown only once. It works for 7 days.
                    </Banner>
                    <div className={styles.link}>
                        <TextField label="Invite link" value={url} readOnly onFocus={e => e.target.select()} />
                        <Button type="button" onClick={() => copy.copy(url)}>
                            Copy link
                        </Button>
                    </div>
                    {copy.copied && (
                        <Banner tone="success" role="status">
                            Link copied.
                        </Banner>
                    )}
                    {copy.error !== null && <ErrorBanner message={copy.error.message} takeFocus />}
                </div>
            )}
        </FormDialog>
    );
}

function AddPeople({ organisation, team, viewer, members, onDone, onClose }: AddPeopleProps) {
    const people = useOrganisationMembers(organisation.id);
    const [chosenMode, setChosenMode] = useState<Mode>("existing");

    const { email_enabled } = useSiteInfo();
    const invites = email_enabled && invitableRoles(viewer.orgRole).includes("member");
    const candidates = people.data === undefined ? [] : candidatesFor(viewer, people.data.members, members);
    const addsExisting = candidates.length > 0;
    const mode = modeFor(addsExisting, invites, chosenMode);

    if (people.error !== null) {
        return (
            <Modal isOpen onClose={onClose} title={dialogTitle(team)}>
                <LoadFailure message={people.error.message} onRetry={people.refetch} />
            </Modal>
        );
    }

    if (people.data === undefined) {
        return (
            <Modal isOpen onClose={onClose} title={dialogTitle(team)}>
                <p className="loading">Loading the organisation&apos;s members...</p>
            </Modal>
        );
    }

    if (!addsExisting && !invites) {
        return (
            <Modal isOpen onClose={onClose} title={dialogTitle(team)}>
                <p>
                    There is nobody else in <bdi>{organisation.name}</bdi> to add yet.{" "}
                    {email_enabled
                        ? "Ask an owner or admin to invite someone new to the organisation."
                        : "Invites are off until this site can send email."}
                </p>
            </Modal>
        );
    }

    const modes: readonly ToggleOption<Mode>[] = [
        {
            value: "existing",
            label: (
                <>
                    Someone in <bdi>{organisation.name}</bdi>
                </>
            ),
        },
        { value: "invite", label: "Invite by email" },
    ];

    const intro = (
        <div className={styles.intro}>
            {addsExisting && invites && (
                <ToggleGroup label="Who to add" options={modes} value={mode} onChange={setChosenMode} />
            )}
            {!addsExisting && (
                <p className={styles.note}>
                    Everyone in <bdi>{organisation.name}</bdi> is already on this team, so invite someone new.
                </p>
            )}
        </div>
    );

    if (mode === "existing") {
        return (
            <ExistingMemberForm team={team} candidates={candidates} intro={intro} onDone={onDone} onClose={onClose} />
        );
    }

    return <InviteForm organisation={organisation} team={team} intro={intro} onDone={onDone} onClose={onClose} />;
}

export function AddPeopleDialog({ open, ...rest }: AddPeopleDialogProps) {
    if (!open) {
        return null;
    }

    return <AddPeople {...rest} />;
}
