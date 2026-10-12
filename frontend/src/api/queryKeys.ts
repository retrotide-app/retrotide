import type {
    ActionItemListQuery,
    AdminOrganisationListQuery,
    AdminUserListQuery,
    AuditLogQuery,
    PageQuery,
} from "../types/api";

export const queryKeys = {
    siteInfo: () => ["site-info"] as const,
    auth: {
        all: ["auth"] as const,
        session: () => ["auth", "session"] as const,
        identities: () => ["auth", "identities"] as const,
        invitation: (token: string) => ["auth", "invitation", token] as const,
        guestClaims: ["auth", "guest-claim"] as const,
        guestClaim: (userId: string) => ["auth", "guest-claim", userId] as const,
        mentions: () => ["auth", "mentions"] as const,
        twoFactor: () => ["auth", "two-factor"] as const,
    },
    admin: {
        all: ["admin"] as const,
        users: {
            all: ["admin", "users"] as const,
            list: (query: AdminUserListQuery) => ["admin", "users", "list", query] as const,
            detail: (userId: string) => ["admin", "users", "detail", userId] as const,
            auditLogPages: (userId: string) => ["admin", "users", "audit-log", userId] as const,
            auditLog: (userId: string, page: PageQuery) => ["admin", "users", "audit-log", userId, page] as const,
        },
        organisations: {
            all: ["admin", "organisations"] as const,
            list: (query: AdminOrganisationListQuery) => ["admin", "organisations", "list", query] as const,
            detail: (orgId: string) => ["admin", "organisations", "detail", orgId] as const,
        },
        invitations: {
            all: ["admin", "invitations"] as const,
            list: (page: PageQuery) => ["admin", "invitations", "list", page] as const,
        },
        settings: () => ["admin", "settings"] as const,
        permissions: () => ["admin", "permissions"] as const,
        stats: () => ["admin", "stats"] as const,
        auditLog: {
            all: ["admin", "audit-log"] as const,
            list: (query: AuditLogQuery) => ["admin", "audit-log", "list", query] as const,
        },
    },
    organisations: {
        all: ["organisations"] as const,
        list: () => ["organisations", "list"] as const,
        scope: (orgId: string) => ["organisations", "org", orgId] as const,
        detail: (orgId: string) => ["organisations", "org", orgId, "detail"] as const,
        members: (orgId: string) => ["organisations", "org", orgId, "members"] as const,
        teams: (orgId: string) => ["organisations", "org", orgId, "teams"] as const,
        invites: (orgId: string) => ["organisations", "org", orgId, "invites"] as const,
        entitlements: (orgId: string) => ["organisations", "org", orgId, "entitlements"] as const,
        savedTemplates: (orgId: string) => ["organisations", "org", orgId, "saved-templates"] as const,
        deletedBoards: (orgId: string) => ["organisations", "org", orgId, "deleted-boards"] as const,
    },
    teams: {
        all: ["teams"] as const,
        scope: (teamId: string) => ["teams", teamId] as const,
        detail: (teamId: string) => ["teams", teamId, "detail"] as const,
        members: (teamId: string) => ["teams", teamId, "members"] as const,
    },
    invites: {
        all: ["invites"] as const,
        lookup: (token: string) => ["invites", "lookup", token] as const,
    },
    boards: {
        all: ["boards"] as const,
        team: (teamId: string) => ["boards", "team", teamId] as const,
        organisations: ["boards", "org"] as const,
        organisation: (orgId: string) => ["boards", "org", orgId] as const,
    },
    boardLive: {
        all: ["board-live"] as const,
        state: (boardId: string) => ["board-live", boardId, "state"] as const,
        mutations: (boardId: string) => ["board-live", boardId, "mutations"] as const,
    },
    boardSharing: (boardId: string) => ["board-sharing", boardId] as const,
    boardPeople: (boardId: string) => ["board-people", boardId] as const,
    actionItems: {
        all: ["action-items"] as const,
        organisation: (orgId: string) => ["action-items", orgId] as const,
        list: (orgId: string, query: ActionItemListQuery) => ["action-items", orgId, query] as const,
    },
    shareLinks: {
        preview: (slug: string) => ["share-links", slug, "preview"] as const,
    },
    boardTemplates: () => ["board-templates"] as const,
    boardTemplate: (key: string) => ["board-templates", key] as const,
    surveyTemplates: () => ["survey-templates"] as const,
};
