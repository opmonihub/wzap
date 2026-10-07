package groups

import (
	"context"

	"github.com/google/uuid"

	"wzap/internal/instance"
	"wzap/internal/model"
)

type InstanceService interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Instance, error)

	CreateGroup(ctx context.Context, id uuid.UUID, input instance.CreateGroupInput) (instance.Group, error)
	GetGroup(ctx context.Context, id uuid.UUID, groupJID string) (instance.Group, error)
	UpdateGroup(ctx context.Context, id uuid.UUID, groupJID string, input instance.UpdateGroupInput) (instance.Group, error)
	SetGroupPhoto(ctx context.Context, id uuid.UUID, groupJID string, image []byte) error
	UpdateGroupParticipants(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error
	GetGroupInvite(ctx context.Context, id uuid.UUID, groupJID string) (string, error)
	ResetGroupInvite(ctx context.Context, id uuid.UUID, groupJID string) (string, error)
	JoinGroup(ctx context.Context, id uuid.UUID, inviteCode string) (string, error)
	LeaveGroup(ctx context.Context, id uuid.UUID, groupJID string) error

	GetJoinedGroups(ctx context.Context, id uuid.UUID, limit int, cursor string) ([]instance.Group, string, error)
	GetGroupInvitePreview(ctx context.Context, id uuid.UUID, inviteCode string) (instance.Group, error)

	GetGroupRequests(ctx context.Context, id uuid.UUID, groupJID string) ([]instance.GroupParticipant, error)
	UpdateGroupRequests(ctx context.Context, id uuid.UUID, groupJID, action string, participants []string) error
	UpdateGroupSettings(ctx context.Context, id uuid.UUID, groupJID string, announce, locked *bool, joinApproval, memberAddMode *string) (instance.Group, error)
}
