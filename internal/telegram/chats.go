package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"regexp"
	"strconv"
	"time"

	"github.com/zelenin/go-tdlib/client"
)

const (
	chatPageLimit       = 100
	allChatsLimit       = int32(1<<31 - 1)
	leaveInterval       = 10 * time.Second
	leaveJitter         = 5 * time.Second
	defaultFloodWait    = 60 * time.Second
	maxFloodWaitRetries = 3
)

var floodWaitPattern = regexp.MustCompile(`(?i)(?:FLOOD_WAIT_|retry after\s+)(\d+)`)

type chatClient interface {
	LoadChats(context.Context, *client.LoadChatsRequest) (*client.Ok, error)
	GetChats(context.Context, *client.GetChatsRequest) (*client.Chats, error)
	GetChat(context.Context, *client.GetChatRequest) (*client.Chat, error)
	GetBasicGroup(context.Context, *client.GetBasicGroupRequest) (*client.BasicGroup, error)
	GetSupergroup(context.Context, *client.GetSupergroupRequest) (*client.Supergroup, error)
	LeaveChat(context.Context, *client.LeaveChatRequest) (*client.Ok, error)
}

func cleanChats(ctx context.Context, tdlibClient chatClient, confirm bool, output io.Writer) error {
	return cleanChatsWithWait(ctx, tdlibClient, confirm, output, waitForContext)
}

type waitFunc func(context.Context, time.Duration) error

func cleanChatsWithWait(ctx context.Context, tdlibClient chatClient, confirm bool, output io.Writer, wait waitFunc) error {
	chats, err := joinedNonAdminChannelsAndGroups(ctx, tdlibClient)
	if err != nil {
		return err
	}
	if len(chats) == 0 {
		_, err := fmt.Fprintln(output, "No joined Telegram channels or groups found where this account is not an administrator.")
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

	for index, chat := range chats {
		if index > 0 {
			delay := leaveInterval + time.Duration(rand.Int64N(int64(leaveJitter)+1))
			if err := wait(ctx, delay); err != nil {
				return err
			}
		}
		if err := leaveChat(ctx, tdlibClient, chat, output, wait); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(output, "Left %d Telegram channels and groups.\n", len(chats))
	return err
}

func leaveChat(ctx context.Context, tdlibClient chatClient, chat *client.Chat, output io.Writer, wait waitFunc) error {
	for retry := 0; ; retry++ {
		_, err := tdlibClient.LeaveChat(ctx, &client.LeaveChatRequest{ChatId: chat.Id})
		if err == nil {
			return nil
		}

		delay, limited := floodWait(err)
		if !limited || retry >= maxFloodWaitRetries {
			return fmt.Errorf("leave Telegram chat %q (%d): %w", chat.Title, chat.Id, err)
		}
		if _, writeErr := fmt.Fprintf(output, "Telegram rate limit reached; waiting %s before retrying %q.\n", delay, chat.Title); writeErr != nil {
			return fmt.Errorf("write Telegram rate-limit notice: %w", writeErr)
		}
		if err := wait(ctx, delay); err != nil {
			return err
		}
	}
}

func floodWait(err error) (time.Duration, bool) {
	var responseError client.ResponseError
	if !errors.As(err, &responseError) || responseError.Err == nil || responseError.Err.Code != 429 {
		return 0, false
	}

	match := floodWaitPattern.FindStringSubmatch(responseError.Err.Message)
	if len(match) != 2 {
		return defaultFloodWait, true
	}
	seconds, parseErr := strconv.ParseInt(match[1], 10, 64)
	if parseErr != nil || seconds <= 0 {
		return defaultFloodWait, true
	}
	return time.Duration(seconds)*time.Second + time.Second, true
}

func waitForContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func joinedNonAdminChannelsAndGroups(ctx context.Context, tdlibClient chatClient) ([]*client.Chat, error) {
	var result []*client.Chat
	seen := make(map[int64]struct{})
	for _, chatList := range []client.ChatList{&client.ChatListMain{}, &client.ChatListArchive{}} {
		for {
			_, err := tdlibClient.LoadChats(ctx, &client.LoadChatsRequest{ChatList: chatList, Limit: chatPageLimit})
			if isChatListExhausted(err) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("load Telegram chats: %w", err)
			}
		}

		chats, err := tdlibClient.GetChats(ctx, &client.GetChatsRequest{ChatList: chatList, Limit: allChatsLimit})
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
			joined, err := isJoinedNonAdminChannelOrGroup(ctx, tdlibClient, chat)
			if err != nil {
				return nil, err
			}
			if joined {
				result = append(result, chat)
			}
		}
	}
	return result, nil
}

func isChatListExhausted(err error) bool {
	var responseError client.ResponseError
	return errors.As(err, &responseError) && responseError.Err != nil && responseError.Err.Code == 404
}

func isJoinedNonAdminChannelOrGroup(ctx context.Context, tdlibClient chatClient, chat *client.Chat) (bool, error) {
	switch chatType := chat.Type.(type) {
	case *client.ChatTypeBasicGroup:
		group, err := tdlibClient.GetBasicGroup(ctx, &client.GetBasicGroupRequest{BasicGroupId: chatType.BasicGroupId})
		if err != nil {
			return false, fmt.Errorf("get Telegram basic group %d: %w", chatType.BasicGroupId, err)
		}
		return isJoinedNonAdminStatus(group.Status), nil
	case *client.ChatTypeSupergroup:
		group, err := tdlibClient.GetSupergroup(ctx, &client.GetSupergroupRequest{SupergroupId: chatType.SupergroupId})
		if err != nil {
			return false, fmt.Errorf("get Telegram supergroup %d: %w", chatType.SupergroupId, err)
		}
		return isJoinedNonAdminStatus(group.Status), nil
	default:
		return false, nil
	}
}

func isJoinedNonAdminStatus(status client.ChatMemberStatus) bool {
	switch typedStatus := status.(type) {
	case *client.ChatMemberStatusMember:
		return true
	case *client.ChatMemberStatusRestricted:
		return typedStatus.IsMember
	default:
		return false
	}
}
