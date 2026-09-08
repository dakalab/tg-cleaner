package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/zelenin/go-tdlib/client"
)

const chatPageLimit = 100

type chatClient interface {
	LoadChats(context.Context, *client.LoadChatsRequest) (*client.Ok, error)
	GetChats(context.Context, *client.GetChatsRequest) (*client.Chats, error)
	GetChat(context.Context, *client.GetChatRequest) (*client.Chat, error)
	GetBasicGroup(context.Context, *client.GetBasicGroupRequest) (*client.BasicGroup, error)
	GetSupergroup(context.Context, *client.GetSupergroupRequest) (*client.Supergroup, error)
	LeaveChat(context.Context, *client.LeaveChatRequest) (*client.Ok, error)
}

func cleanChats(ctx context.Context, tdlibClient chatClient, confirm bool, output io.Writer) error {
	chats, err := joinedChannelsAndGroups(ctx, tdlibClient)
	if err != nil {
		return err
	}
	if len(chats) == 0 {
		_, err := fmt.Fprintln(output, "No joined Telegram channels or groups found.")
		return err
	}

	for _, chat := range chats {
		if _, err := fmt.Fprintf(output, "%s (ID: %d)\n", chat.Title, chat.Id); err != nil {
			return fmt.Errorf("write Telegram chat: %w", err)
		}
	}
	if !confirm {
		_, err := fmt.Fprintf(output, "Found %d chats. Re-run with --confirm to leave them.\n", len(chats))
		return err
	}

	for _, chat := range chats {
		if _, err := tdlibClient.LeaveChat(ctx, &client.LeaveChatRequest{ChatId: chat.Id}); err != nil {
			return fmt.Errorf("leave Telegram chat %q (%d): %w", chat.Title, chat.Id, err)
		}
	}
	_, err = fmt.Fprintf(output, "Left %d Telegram channels and groups.\n", len(chats))
	return err
}

func joinedChannelsAndGroups(ctx context.Context, tdlibClient chatClient) ([]*client.Chat, error) {
	var result []*client.Chat
	seen := make(map[int64]struct{})
	for _, chatList := range []client.ChatList{&client.ChatListMain{}, &client.ChatListArchive{}} {
		for {
			_, err := tdlibClient.LoadChats(ctx, &client.LoadChatsRequest{ChatList: chatList, Limit: chatPageLimit})
			exhausted := isChatListExhausted(err)
			if err != nil && !exhausted {
				return nil, fmt.Errorf("load Telegram chats: %w", err)
			}

			chats, err := tdlibClient.GetChats(ctx, &client.GetChatsRequest{ChatList: chatList, Limit: chatPageLimit})
			if err != nil {
				return nil, fmt.Errorf("get Telegram chats: %w", err)
			}
			for _, chatID := range chats.ChatIds {
				if _, ok := seen[chatID]; ok {
					continue
				}
				seen[chatID] = struct{}{}
				chat, err := tdlibClient.GetChat(ctx, &client.GetChatRequest{ChatId: chatID})
				if err != nil {
					return nil, fmt.Errorf("get Telegram chat %d: %w", chatID, err)
				}
				joined, err := isJoinedChannelOrGroup(ctx, tdlibClient, chat)
				if err != nil {
					return nil, err
				}
				if joined {
					result = append(result, chat)
				}
			}
			if exhausted {
				break
			}
		}
	}
	return result, nil
}

func isChatListExhausted(err error) bool {
	var responseError client.ResponseError
	return errors.As(err, &responseError) && responseError.Err != nil && responseError.Err.Code == 404
}

func isJoinedChannelOrGroup(ctx context.Context, tdlibClient chatClient, chat *client.Chat) (bool, error) {
	switch chatType := chat.Type.(type) {
	case *client.ChatTypeBasicGroup:
		group, err := tdlibClient.GetBasicGroup(ctx, &client.GetBasicGroupRequest{BasicGroupId: chatType.BasicGroupId})
		if err != nil {
			return false, fmt.Errorf("get Telegram basic group %d: %w", chatType.BasicGroupId, err)
		}
		return isJoinedStatus(group.Status), nil
	case *client.ChatTypeSupergroup:
		group, err := tdlibClient.GetSupergroup(ctx, &client.GetSupergroupRequest{SupergroupId: chatType.SupergroupId})
		if err != nil {
			return false, fmt.Errorf("get Telegram supergroup %d: %w", chatType.SupergroupId, err)
		}
		return isJoinedStatus(group.Status), nil
	default:
		return false, nil
	}
}

func isJoinedStatus(status client.ChatMemberStatus) bool {
	switch typedStatus := status.(type) {
	case *client.ChatMemberStatusCreator:
		return typedStatus.IsMember
	case *client.ChatMemberStatusAdministrator, *client.ChatMemberStatusMember:
		return true
	default:
		return false
	}
}
