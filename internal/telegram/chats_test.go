package telegram

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/zelenin/go-tdlib/client"
)

type fakeChatClient struct {
	chatIDs     []int64
	chats       map[int64]*client.Chat
	basicGroups map[int64]*client.BasicGroup
	supergroups map[int64]*client.Supergroup
	leftChatIDs []int64
	leaveErrors []error
}

func (fake *fakeChatClient) LoadChats(context.Context, *client.LoadChatsRequest) (*client.Ok, error) {
	return nil, client.ResponseError{Err: &client.Error{Code: 404}}
}

func (fake *fakeChatClient) GetChats(_ context.Context, _ *client.GetChatsRequest) (*client.Chats, error) {
	return &client.Chats{ChatIds: fake.chatIDs}, nil
}

func (fake *fakeChatClient) GetChat(_ context.Context, request *client.GetChatRequest) (*client.Chat, error) {
	chat, ok := fake.chats[request.ChatId]
	if !ok {
		return nil, fmt.Errorf("chat not found")
	}
	return chat, nil
}

func (fake *fakeChatClient) GetBasicGroup(_ context.Context, request *client.GetBasicGroupRequest) (*client.BasicGroup, error) {
	return fake.basicGroups[request.BasicGroupId], nil
}

func (fake *fakeChatClient) GetSupergroup(_ context.Context, request *client.GetSupergroupRequest) (*client.Supergroup, error) {
	return fake.supergroups[request.SupergroupId], nil
}

func (fake *fakeChatClient) LeaveChat(_ context.Context, request *client.LeaveChatRequest) (*client.Ok, error) {
	fake.leftChatIDs = append(fake.leftChatIDs, request.ChatId)
	if len(fake.leaveErrors) > 0 {
		err := fake.leaveErrors[0]
		fake.leaveErrors = fake.leaveErrors[1:]
		return nil, err
	}
	return &client.Ok{}, nil
}

func TestCleanChatsPreviewsJoinedNonAdminChannelsAndGroups(t *testing.T) {
	tdlibClient := newFakeChatClient()
	var output bytes.Buffer

	if err := cleanChatsWithWait(context.Background(), tdlibClient, false, &output, noWait); err != nil {
		t.Fatalf("cleanChats() error = %v", err)
	}

	if len(tdlibClient.leftChatIDs) != 0 {
		t.Fatalf("LeaveChat calls = %v, want none", tdlibClient.leftChatIDs)
	}
	want := "Group (ID: 1)\nRestricted group (ID: 5)\nFound 2 chats. Re-run with --confirm to leave them.\n"
	if output.String() != want {
		t.Errorf("output = %q, want %q", output.String(), want)
	}
}

func TestCleanChatsLeavesJoinedNonAdminChannelsAndGroupsWhenConfirmed(t *testing.T) {
	tdlibClient := newFakeChatClient()
	var output bytes.Buffer

	if err := cleanChatsWithWait(context.Background(), tdlibClient, true, &output, noWait); err != nil {
		t.Fatalf("cleanChats() error = %v", err)
	}

	if got, want := fmt.Sprint(tdlibClient.leftChatIDs), "[1 5]"; got != want {
		t.Errorf("left chat IDs = %s, want %s", got, want)
	}
}

func TestJoinedNonAdminChannelsAndGroupsExcludesPrivateAdminAndLeftChats(t *testing.T) {
	tdlibClient := newFakeChatClient()

	chats, err := joinedNonAdminChannelsAndGroups(context.Background(), tdlibClient)
	if err != nil {
		t.Fatalf("joinedNonAdminChannelsAndGroups() error = %v", err)
	}
	if got, want := len(chats), 2; got != want {
		t.Fatalf("joined chat count = %d, want %d", got, want)
	}
	if chats[0].Id != 1 || chats[1].Id != 5 {
		t.Errorf("joined chat IDs = [%d %d], want [1 5]", chats[0].Id, chats[1].Id)
	}
}

func TestCleanChatsRetriesAfterTelegramRateLimit(t *testing.T) {
	tdlibClient := newFakeChatClient()
	tdlibClient.chatIDs = []int64{1}
	tdlibClient.leaveErrors = []error{
		client.ResponseError{Err: &client.Error{Code: 429, Message: "Too Many Requests: retry after 2"}},
	}
	var output bytes.Buffer
	var waits []time.Duration
	wait := func(_ context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}

	if err := cleanChatsWithWait(context.Background(), tdlibClient, true, &output, wait); err != nil {
		t.Fatalf("cleanChats() error = %v", err)
	}

	if got, want := fmt.Sprint(tdlibClient.leftChatIDs), "[1 1]"; got != want {
		t.Errorf("left chat IDs = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(waits), "[3s]"; got != want {
		t.Errorf("waits = %s, want %s", got, want)
	}
}

func TestFloodWaitUsesSafeDefaultForRateLimitWithoutDuration(t *testing.T) {
	delay, limited := floodWait(client.ResponseError{Err: &client.Error{Code: 429, Message: "Too Many Requests"}})
	if !limited {
		t.Fatal("floodWait() limited = false, want true")
	}
	if delay != defaultFloodWait {
		t.Errorf("floodWait() delay = %s, want %s", delay, defaultFloodWait)
	}
}

func noWait(context.Context, time.Duration) error {
	return nil
}

func newFakeChatClient() *fakeChatClient {
	return &fakeChatClient{
		chatIDs: []int64{1, 2, 3, 4, 5, 6},
		chats: map[int64]*client.Chat{
			1: {Id: 1, Title: "Group", Type: &client.ChatTypeBasicGroup{BasicGroupId: 10}},
			2: {Id: 2, Title: "Admin channel", Type: &client.ChatTypeSupergroup{SupergroupId: 20, IsChannel: true}},
			3: {Id: 3, Title: "Private", Type: &client.ChatTypePrivate{}},
			4: {Id: 4, Title: "Left group", Type: &client.ChatTypeSupergroup{SupergroupId: 40}},
			5: {Id: 5, Title: "Restricted group", Type: &client.ChatTypeSupergroup{SupergroupId: 50}},
			6: {Id: 6, Title: "Owned group", Type: &client.ChatTypeBasicGroup{BasicGroupId: 60}},
		},
		basicGroups: map[int64]*client.BasicGroup{
			10: {Status: &client.ChatMemberStatusMember{}},
			60: {Status: &client.ChatMemberStatusCreator{IsMember: true}},
		},
		supergroups: map[int64]*client.Supergroup{
			20: {Status: &client.ChatMemberStatusAdministrator{}},
			40: {Status: &client.ChatMemberStatusLeft{}},
			50: {Status: &client.ChatMemberStatusRestricted{IsMember: true}},
		},
	}
}
