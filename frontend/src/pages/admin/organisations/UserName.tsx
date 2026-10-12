import { TextLink } from "../../../components/TextLink/TextLink";
import { useAuth } from "../../../hooks/useAuth";

interface UserNameProps {
    userId: string;
    name: string;
}

export function UserName({ userId, name }: UserNameProps) {
    const { permissions } = useAuth();

    if (!permissions.includes("view_users")) {
        return <bdi>{name}</bdi>;
    }

    return (
        <TextLink to={`/admin/users/${encodeURIComponent(userId)}`}>
            <bdi>{name}</bdi>
        </TextLink>
    );
}
