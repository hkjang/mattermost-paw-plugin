package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

type Plugin struct {
	plugin.MattermostPlugin

	client *pluginapi.Client
	router *mux.Router

	configurationLock sync.RWMutex
	configuration     *configuration

	botLock sync.RWMutex
	bot     botAccount
}

type botAccount struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	UserID      string `json:"user_id"`
	Active      bool   `json:"active"`
	LastError   string `json:"last_error,omitempty"`
	UpdatedAt   int64  `json:"updated_at"`
}

type incomingMessage struct {
	Channel *model.Channel
	User    *model.User
	Prompt  string
	RootID  string
	IsDM    bool
}

func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)
	if err := p.OnConfigurationChange(); err != nil {
		return err
	}
	p.router = p.initRouter()
	if err := p.ensureBot(); err != nil {
		p.API.LogError("Failed to ensure paw bot during activation", "error", err)
	}
	return nil
}

func (p *Plugin) OnDeactivate() error {
	return nil
}

func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	if post == nil || post.UserId == "" || p.isBotUserID(post.UserId) {
		return
	}
	if post.GetProp("from_bot") != nil || post.GetProp("from_plugin") != nil || post.GetProp("from_webhook") != nil {
		return
	}
	if post.RemoteId != nil && *post.RemoteId != "" {
		return
	}

	go func() {
		if err := p.handlePostedMessage(post); err != nil {
			p.API.LogError("Failed to process paw post trigger", "error", err, "post_id", post.Id)
		}
	}()
}

func (p *Plugin) handlePostedMessage(post *model.Post) error {
	cfg, err := p.getRuntimeConfiguration()
	if err != nil {
		return err
	}
	account, err := p.getOrEnsureBot()
	if err != nil {
		return err
	}

	channel, appErr := p.API.GetChannel(post.ChannelId)
	if appErr != nil {
		return fmt.Errorf("failed to get channel: %w", appErr)
	}

	// DM 梨꾨꼸??寃쎌슦, paw 遊뉗씠 ?대떦 DM??硫ㅻ쾭?몄? ?뺤씤.
	// ?ㅻⅨ ?뚮윭洹몄씤(?? OCS)??留뚮뱺 遊뉕낵??DM? 臾댁떆?쒕떎.
	if channel.Type == model.ChannelTypeDirect {
		if account.UserID == "" {
			return nil
		}
		if !strings.Contains(channel.Name, account.UserID) {
			return nil
		}
	}

	user, appErr := p.API.GetUser(post.UserId)
	if appErr != nil {
		return fmt.Errorf("failed to get user: %w", appErr)
	}
	if user.IsBot {
		return nil
	}

	prompt, triggered := extractPrompt(cfg.BotUsername, account.UserID, channel, post.Message)
	if !triggered {
		return nil
	}
	message := incomingMessage{
		Channel: channel,
		User:    user,
		Prompt:  strings.TrimSpace(prompt),
		RootID:  responseRootID(post, channel),
		IsDM:    channel.Type == model.ChannelTypeDirect,
	}
	if len(post.FileIds) > 0 {
		return p.postText(channel.Id, message.RootID, "泥⑤? ?뚯씪? ?꾩쭅 吏?먰븯吏 ?딆뒿?덈떎.")
	}
	if message.Prompt == "" {
		return p.postText(channel.Id, message.RootID, buildUsageMessage(cfg.BotUsername))
	}
	return p.processPawMessage(context.Background(), cfg, message)
}

func extractPrompt(botUsername, botUserID string, channel *model.Channel, message string) (string, bool) {
	message = strings.TrimSpace(message)
	if channel == nil || message == "" {
		return "", false
	}

	mentionPatterns := []string{"@" + strings.ToLower(botUsername)}
	if botUserID != "" {
		mentionPatterns = append(mentionPatterns, "@"+strings.ToLower(botUserID))
	}

	lowerMessage := strings.ToLower(message)
	mentionFound := false
	for _, mention := range mentionPatterns {
		if strings.Contains(lowerMessage, mention) {
			mentionFound = true
			break
		}
	}

	switch channel.Type {
	case model.ChannelTypeDirect:
		return stripMentions(message, botUsername, botUserID), true
	case model.ChannelTypeGroup:
		if !mentionFound {
			return "", false
		}
		return stripMentions(message, botUsername, botUserID), true
	default:
		if !mentionFound {
			return "", false
		}
		return stripMentions(message, botUsername, botUserID), true
	}
}

func stripMentions(message, botUsername, botUserID string) string {
	tokens := []string{regexp.QuoteMeta(botUsername)}
	if botUserID != "" {
		tokens = append(tokens, regexp.QuoteMeta(botUserID))
	}
	pattern := regexp.MustCompile(`(?i)@(` + strings.Join(tokens, "|") + `)\b`)
	return strings.TrimSpace(pattern.ReplaceAllString(message, ""))
}

func responseRootID(post *model.Post, channel *model.Channel) string {
	if post == nil || channel == nil {
		return ""
	}
	if post.RootId != "" {
		return post.RootId
	}
	return post.Id
}

func (p *Plugin) processPawMessage(ctx context.Context, cfg *runtimeConfiguration, msg incomingMessage) error {
	// domainUserID: QwenPaw ?쒕툕?꾨찓?몄슜 (sj.lee ??sj-lee)
	domainUserID, err := resolveMappedUserID(msg.User, cfg)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "?ъ슜??ID瑜??꾨찓?몄쑝濡?蹂?섑븷 ???놁뒿?덈떎. 愿由ъ옄?먭쾶 臾몄쓽?댁＜?몄슂.")
	}
	// hubUserID: JupyterHub API???먮낯 username (sj.lee 洹몃?濡?
	hubUserID := strings.ToLower(strings.TrimSpace(msg.User.Username))

	command := classifyUserCommand(msg.Prompt)
	switch command {
	case "start":
		if !cfg.AllowUserStartServer {
			return p.postText(msg.Channel.Id, msg.RootID, "?쒕쾭 ?쒖옉 湲곕뒫??鍮꾪솢?깊솕?섏뼱 ?덉뒿?덈떎.")
		}
		return p.startUserServer(ctx, cfg, msg.Channel.Id, msg.RootID, hubUserID)
	case "stop":
		if !cfg.AllowUserStopServer {
			return p.postText(msg.Channel.Id, msg.RootID, "?쒕쾭 以묒? 湲곕뒫??鍮꾪솢?깊솕?섏뼱 ?덉뒿?덈떎.")
		}
		return p.stopUserServer(ctx, cfg, msg.Channel.Id, msg.RootID, hubUserID)
	case "status":
		status, err := p.getUserServerStatus(ctx, cfg, hubUserID)
		if err != nil {
			return p.postText(msg.Channel.Id, msg.RootID, "媛쒖씤 ?먯씠?꾪듃 ?쒕쾭 ?곹깭瑜??뺤씤?????놁뒿?덈떎.")
		}
		if status.Ready {
			return p.postText(msg.Channel.Id, msg.RootID, "媛쒖씤 ?먯씠?꾪듃 ?쒕쾭媛 耳쒖졇 ?덉뒿?덈떎.")
		}
		return p.postText(msg.Channel.Id, msg.RootID, "媛쒖씤 ?먯씠?꾪듃 ?쒕쾭媛 爰쇱졇 ?덉뒿?덈떎.")
	case "session_reset":
		return p.handleSessionReset(ctx, cfg, msg, domainUserID)
	case "session_abort":
		return p.handleSessionAbort(ctx, cfg, msg, domainUserID)
	case "session_info":
		return p.handleSessionInfo(msg, domainUserID)
	}

	status, err := p.getUserServerStatus(ctx, cfg, hubUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "媛쒖씤 ?먯씠?꾪듃 ?쒕쾭 ?곹깭瑜??뺤씤?????놁뒿?덈떎.")
	}
	if !status.Ready {
		if !cfg.AutoStartServer {
			return p.postText(msg.Channel.Id, msg.RootID, "?쒕쾭瑜?癒쇱? 耳쒖＜?몄슂. `耳쒖쨾` ?먮뒗 `?쒕쾭 耳쒖쨾`?쇨퀬 蹂대궡硫??쒖옉?????덉뒿?덈떎.")
		}
		if err := p.startUserServerAndWait(ctx, cfg, hubUserID); err != nil {
			return p.postText(msg.Channel.Id, msg.RootID, "媛쒖씤 ?먯씠?꾪듃 ?쒕쾭瑜??쒖옉?????놁뒿?덈떎.")
		}
	}

	baseURL := buildUserQwenPawURL(domainUserID, cfg.BaseDomainSuffix)
	sessionID, err := p.getOrCreateSessionID(ctx, cfg, baseURL, msg, domainUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, userFacingQwenPawError(err))
	}
	output, err := p.streamQwenPawMessage(ctx, cfg, baseURL, sessionID, domainUserID, msg.Channel.Id, msg.RootID, msg.Prompt)
	if err != nil {
		var callErr *qwenPawCallError
		if errors.As(err, &callErr) && callErr.StatusCode == http.StatusNotFound {
			if newSessionID, resetErr := p.resetSessionID(ctx, cfg, baseURL, msg, domainUserID); resetErr == nil {
				output, err = p.streamQwenPawMessage(ctx, cfg, baseURL, newSessionID, domainUserID, msg.Channel.Id, msg.RootID, msg.Prompt)
			}
		}
	}
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, userFacingQwenPawError(err))
	}
	if strings.TrimSpace(output) == "" {
		output = "?묐떟??鍮꾩뼱 ?덉뒿?덈떎."
	}
	return nil
}

func classifyUserCommand(prompt string) string {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	normalized = strings.Join(strings.Fields(normalized), " ")
	switch normalized {
	case "耳쒖쨾", "?쒕쾭 耳쒖쨾":
		return "start"
	case "爰쇱쨾", "?쒕쾭 爰쇱쨾":
		return "stop"
	case "status", "/status":
		return "status"
	case "reset", "/reset", "new session", "/new":
		return "session_reset"
	case "以묐떒", "痍⑥냼", "?몄뀡 以묐떒", "?몄뀡 痍⑥냼", "abort", "/abort", "cancel", "/cancel", "stop":
		return "session_abort"
	case "?몄뀡 ?뺣낫", "?몄뀡 ?곹깭", "?몄뀡 蹂닿린", "?몄뀡", "session", "/session":
		return "session_info"
	default:
		return ""
	}
}

func buildUsageMessage(botUsername string) string {
	return strings.Join([]string{
		fmt.Sprintf("`@%s` ?ㅼ뿉 吏덈Ц???낅젰?섍굅?? 1:1 DM?먯꽌??諛붾줈 吏덈Ц??蹂대궡二쇱꽭??", botUsername),
		"",
		"?쒕쾭 ?쒖뼱: `耳쒖쨾`, `?쒕쾭 耳쒖쨾`, `爰쇱쨾`, `?쒕쾭 爰쇱쨾`, `?곹깭 ?뚮젮以?",
		"?몄뀡 愿由? `?몄뀡 ?뺣낫`, `?몄뀡 珥덇린?? (紐⑤뜽 ?ㅼ젙 臾몄젣 ?깆쑝濡??묐떟????????, `以묐떒` (吏꾪뻾 以??묒뾽 痍⑥냼)",
	}, "\n")
}

const sessionResetHint = "?꾩옱 ?몄뀡??留앷?議뚯쓣 ???덉뒿?덈떎. `?몄뀡 珥덇린???쇨퀬 蹂대궡硫????몄뀡???쒖옉?????덉뒿?덈떎."

func (p *Plugin) handleSessionReset(ctx context.Context, cfg *runtimeConfiguration, msg incomingMessage, domainUserID string) error {
	baseURL := buildUserQwenPawURL(domainUserID, cfg.BaseDomainSuffix)
	previous, _ := p.getStoredSessionID(msg, domainUserID)
	newSessionID, err := p.resetSessionID(ctx, cfg, baseURL, msg, domainUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "?몄뀡 珥덇린?붿뿉 ?ㅽ뙣?덉뒿?덈떎: "+userFacingQwenPawError(err))
	}
	body := fmt.Sprintf("???몄뀡???쒖옉?덉뒿?덈떎.\n- ???몄뀡 ID: `%s`", newSessionID)
	if previous != "" && previous != newSessionID {
		body += fmt.Sprintf("\n- ?댁쟾 ?몄뀡 ID: `%s` (?뺣━??", previous)
	}
	body += "\n\n?ㅼ떆 吏덈Ц??蹂대궡二쇱꽭??"
	return p.postText(msg.Channel.Id, msg.RootID, body)
}

func (p *Plugin) handleSessionAbort(ctx context.Context, cfg *runtimeConfiguration, msg incomingMessage, domainUserID string) error {
	sessionID, err := p.getStoredSessionID(msg, domainUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "?몄뀡 ?뺣낫瑜?遺덈윭?ㅼ? 紐삵뻽?듬땲??")
	}
	if sessionID == "" {
		return p.postText(msg.Channel.Id, msg.RootID, "????붿뿉??吏꾪뻾 以묒씤 ?몄뀡???놁뒿?덈떎.")
	}
	baseURL := buildUserQwenPawURL(domainUserID, cfg.BaseDomainSuffix)
	if err := p.abortQwenPawSession(ctx, cfg, baseURL, sessionID); err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "?몄뀡??以묐떒?섏? 紐삵뻽?듬땲?? "+userFacingQwenPawError(err))
	}
	return p.postText(msg.Channel.Id, msg.RootID, fmt.Sprintf("?몄뀡 `%s`??吏꾪뻾 以??묒뾽??以묐떒?덉뒿?덈떎.", sessionID))
}

func (p *Plugin) handleSessionInfo(msg incomingMessage, domainUserID string) error {
	sessionID, err := p.getStoredSessionID(msg, domainUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "?몄뀡 ?뺣낫瑜?遺덈윭?ㅼ? 紐삵뻽?듬땲??")
	}
	scope := "DM"
	if msg.Channel != nil && !msg.IsDM {
		if msg.RootID != "" {
			scope = fmt.Sprintf("?ㅻ젅??猷⑦듃 `%s`)", msg.RootID)
		} else {
			channelName := msg.Channel.Name
			if channelName == "" {
				channelName = msg.Channel.Id
			}
			scope = "梨꾨꼸 `" + channelName + "`"
		}
	}
	if sessionID == "" {
		body := fmt.Sprintf("??%s?먮뒗 ?꾩쭅 ?몄뀡???놁뒿?덈떎. 泥?吏덈Ц??蹂대궡硫??먮룞?쇰줈 ?앹꽦?⑸땲??", scope)
		return p.postText(msg.Channel.Id, msg.RootID, body)
	}
	body := strings.Join([]string{
		fmt.Sprintf("- ?ㅼ퐫?? %s", scope),
		fmt.Sprintf("- ?몄뀡 ID: `%s`", sessionID),
		"",
		"紐낅졊?? `?몄뀡 珥덇린?? (???몄뀡), `以묐떒` (吏꾪뻾 以??묒뾽 痍⑥냼)",
	}, "\n")
	return p.postText(msg.Channel.Id, msg.RootID, body)
}

func (p *Plugin) postText(channelID, rootID, message string) error {
	account, err := p.getOrEnsureBot()
	if err != nil {
		return err
	}
	if err := p.ensureBotInChannel(channelID, account.UserID); err != nil {
		return p.postAsPluginFallback(channelID, rootID, "遊뉗쓣 梨꾨꼸??珥덈??섍굅??沅뚰븳???뺤씤?댁빞 ?⑸땲??")
	}
	_, appErr := p.API.CreatePost(&model.Post{
		UserId:    account.UserID,
		ChannelId: channelID,
		RootId:    rootID,
		Type:      "custom_paw_bot",
		Message:   strings.TrimSpace(message),
		Props: map[string]any{
			"from_bot": "true",
			"paw_bot":  "true",
		},
	})
	if appErr != nil {
		return fmt.Errorf("failed to create paw post: %w", appErr)
	}
	return nil
}

func (p *Plugin) postLongText(channelID, rootID, message string) error {
	parts := splitMessage(message, defaultMaxMattermostMessageLength)
	for _, part := range parts {
		if err := p.postText(channelID, rootID, part); err != nil {
			return err
		}
	}
	return nil
}

func (p *Plugin) postAsPluginFallback(channelID, rootID, message string) error {
	_, appErr := p.API.CreatePost(&model.Post{
		ChannelId: channelID,
		RootId:    rootID,
		Message:   strings.TrimSpace(message),
		Props: map[string]any{
			"from_plugin": "true",
			"paw_bot":     "true",
		},
	})
	if appErr != nil {
		return fmt.Errorf("failed to create fallback post: %w", appErr)
	}
	return nil
}
