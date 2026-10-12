package audit

type (
	Action string

	TargetType string

	ActorKind string
)

const (
	ActionOrgCreated              Action = "org_created"
	ActionOrgUpdated              Action = "org_updated"
	ActionOrgDeleted              Action = "org_deleted"
	ActionOrgRestored             Action = "org_restored"
	ActionOrgOwnershipTransferred Action = "org_ownership_transferred"

	ActionMemberInvited     Action = "member_invited"
	ActionInviteRevoked     Action = "invite_revoked"
	ActionMemberJoined      Action = "member_joined"
	ActionMemberRoleChanged Action = "member_role_changed"
	ActionMemberRemoved     Action = "member_removed"

	ActionTeamCreated           Action = "team_created"
	ActionTeamArchived          Action = "team_archived"
	ActionTeamDeleted           Action = "team_deleted"
	ActionTeamMemberAdded       Action = "team_member_added"
	ActionTeamMemberRoleChanged Action = "team_member_role_changed"
	ActionTeamMemberRemoved     Action = "team_member_removed"

	ActionBoardCreated         Action = "board_created"
	ActionBoardArchived        Action = "board_archived"
	ActionBoardDeleted         Action = "board_deleted"
	ActionBoardRestored        Action = "board_restored"
	ActionBoardSharingChanged  Action = "board_sharing_changed"
	ActionBoardPasswordChanged Action = "board_password_changed"
	ActionBoardLinkRotated     Action = "board_link_rotated"
	ActionBoardClosed          Action = "board_closed"

	ActionSubscriptionChanged    Action = "subscription_changed"
	ActionEntitlementOverrideSet Action = "entitlement_override_set"
	ActionGuestClaimed           Action = "guest_claimed"
	ActionIdentityLinked         Action = "identity_linked"
	ActionIdentityUnlinked       Action = "identity_unlinked"
	ActionInvitationAccepted     Action = "invitation_accepted"
	ActionTwoFactorEnabled       Action = "two_factor_enabled"
	ActionTwoFactorDisabled      Action = "two_factor_disabled"

	ActionSetRole               Action = "set_role"
	ActionRemoveRole            Action = "remove_role"
	ActionBanUser               Action = "ban_user"
	ActionUnbanUser             Action = "unban_user"
	ActionLockUser              Action = "lock_user"
	ActionUnlockUser            Action = "unlock_user"
	ActionDeleteUser            Action = "delete_user"
	ActionResetPassword         Action = "reset_password"
	ActionSetUserEmail          Action = "set_user_email"
	ActionVerifyUserEmail       Action = "verify_user_email"
	ActionUnverifyUserEmail     Action = "unverify_user_email"
	ActionSetDisplayName        Action = "set_display_name"
	ActionForceLogout           Action = "force_logout"
	ActionUpdateSettings        Action = "update_settings"
	ActionSendTestEmail         Action = "send_test_email"
	ActionUpdateRolePermissions Action = "update_role_permissions"
	ActionInviteUser            Action = "invite_user"
	ActionResendInvitation      Action = "resend_invitation"
	ActionCancelInvitation      Action = "cancel_invitation"
)

const (
	TargetUser       TargetType = "user"
	TargetSettings   TargetType = "settings"
	TargetRole       TargetType = "role"
	TargetIdentity   TargetType = "identity"
	TargetInvitation TargetType = "invitation"

	TargetOrganisation TargetType = "organisation"
	TargetTeam         TargetType = "team"
	TargetInvite       TargetType = "invite"
	TargetBoard        TargetType = "board"
)

const (
	ActorUser   ActorKind = "user"
	ActorStaff  ActorKind = "staff"
	ActorSystem ActorKind = "system"
)
