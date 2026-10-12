import { FormField } from "../../../components/FormField/FormField";
import { Select } from "../../../components/Select/Select";
import { ORG_ROLE_LABELS } from "../../../domain/organisationRoles";
import type { AssignableOrgRole, TeamResponse } from "../../../types/api";

interface InviteOptionsProps {
    roles: readonly AssignableOrgRole[];
    role: AssignableOrgRole;
    onRoleChange: (role: AssignableOrgRole) => void;
    teams: readonly TeamResponse[];
    teamId: string;
    onTeamChange: (teamId: string) => void;
    teamError?: string | null;
}

export function InviteOptions({
    roles,
    role,
    onRoleChange,
    teams,
    teamId,
    onTeamChange,
    teamError = null,
}: InviteOptionsProps) {
    return (
        <>
            <FormField label="Role">
                {control => (
                    <Select
                        {...control}
                        value={role}
                        onChange={e => {
                            const picked = roles.find(option => option === e.target.value);

                            if (picked !== undefined) {
                                onRoleChange(picked);
                            }
                        }}
                    >
                        {roles.map(option => (
                            <option key={option} value={option}>
                                {ORG_ROLE_LABELS[option]}
                            </option>
                        ))}
                    </Select>
                )}
            </FormField>
            <FormField label="Team" hint="They also join this team as a member." error={teamError}>
                {control => (
                    <Select {...control} value={teamId} onChange={e => onTeamChange(e.target.value)}>
                        <option value="">No team</option>
                        {teams.map(team => (
                            <option key={team.id} value={team.id}>
                                {team.name}
                            </option>
                        ))}
                    </Select>
                )}
            </FormField>
        </>
    );
}
