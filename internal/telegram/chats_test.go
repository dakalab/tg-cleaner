package telegram

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/zelenin/go-tdlib/client"
)

type fakeChatClient struct {
	chatIDs     []int64
	chats       map[int64]*client.Chat
	basicGroups map[int64]*client.BasicGroup
	supergroups map[int64]*client.Supergroup
	leftChatIDs []int64
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
	return &client.Ok{}, nil
}

func TestCleanChatsPreviewsJoinedChannelsAndGroups(t *testing.T) {
	tdlibClient := newFakeChatClient()
	var output bytes.Buffer

	if err := cleanChats(context.Background(), tdlibClient, false, &output); err != nil {
		t.Fatalf("cleanChats() error = %v", err)
	}

	if len(tdlibClient.leftChatIDs) != 0 {
		t.Fatalf("LeaveChat calls = %v, want none", tdlibClient.leftChatIDs)
	}
	want := "Group (ID: 1)\nChannel (ID: 2)\nFound 2 chats. Re-run with --confirm to leave them.\n"
	if output.String() != want {
		t.Errorf("output = %q, want %q", output.String(), want)
	}
}

func TestCleanChatsLeavesJoinedChannelsAndGroupsWhenConfirmed(t *testing.T) {
	tdlibClient := newFakeChatClient()
	var output bytes.Buffer

	if err := cleanChats(context.Background(), tdlibClient, true, &output); err != nil {
		t.Fatalf("cleanChats() error = %v", err)
	}

	if got, want := fmt.Sprint(tdlibClient.leftChatIDs), "[1 2]"; got != want {
		t.Errorf("left chat IDs = %s, want %s", got, want)
	}
}

func TestJoinedChannelsAndGroupsExcludesPrivateAndLeftChats(t *testing.T) {
	tdlibClient := newFakeChatClient()

	chats, err := joinedChannelsAndGroups(context.Background(), tdlibClient)
	if err != nil {
		t.Fatalf("joinedChannelsAndGroups() error = %v", err)
	}
	if got, want := len(chats), 2; got != want {
		t.Fatalf("joined chat count = %d, want %d", got, want)
	}
	if chats[0].Id != 1 || chats[1].Id != 2 {
		t.Errorf("joined chat IDs = [%d %d], want [1 2]", chats[0].Id, chats[1].Id)
	}
}

func newFakeChatClient() *fakeChatClient {
	return &fakeChatClient{
		chatIDs: []int64{1, 2, 3, 4},
		chats: map[int64]*client.Chat{
			1: {Id: 1, Title: "Group", Type: &client.ChatTypeBasicGroup{BasicGroupId: 10}},
			2: {Id: 2, Title: "Channel", Type: &client.ChatTypeSupergroup{SupergroupId: 20, IsChannel: true}},
			3: {Id: 3, Title: "Private", Type: &client.ChatTypePrivate{}},
			4: {Id: 4, Title: "Left group", Type: &client.ChatTypeSupergroup{SupergroupId: 40}},
		},
		basicGroups: map[int64]*client.BasicGroup{
			10: {Status: &client.ChatMemberStatusMember{}},
		},
		supergroups: map[int64]*client.Supergroup{
			20: {Status: &client.ChatMemberStatusAdministrator{}},
			40: {Status: &client.ChatMemberStatusLeft{}},
		},
	}
}
