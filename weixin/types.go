package weixin

// ilink 机器人 HTTP 协议消息类型定义

const (
	messageTypeUser = 1
	messageTypeBot  = 2

	messageItemText  = 1
	messageItemImage = 2
	messageItemVoice = 3
	messageItemFile  = 4
	messageItemVideo = 5

	messageStateFinish = 2

	sessionExpiredErrcode = -14

	uploadMediaImage = 1
	uploadMediaVideo = 2
	uploadMediaFile  = 3

	typingStatusStart = 1
	typingStatusStop  = 2
)

type baseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

// --- 请求/响应结构 ---

type getUpdatesReq struct {
	GetUpdatesBuf string   `json:"get_updates_buf"`
	BaseInfo      baseInfo `json:"base_info"`
}

type getUpdatesResp struct {
	Ret                  int             `json:"ret"`
	Errcode              int             `json:"errcode"`
	Errmsg               string          `json:"errmsg"`
	Msgs                 []weixinMessage `json:"msgs"`
	GetUpdatesBuf        string          `json:"get_updates_buf"`
	LongpollingTimeoutMs int             `json:"longpolling_timeout_ms"`
}

type sendMessageReq struct {
	Msg      weixinOutboundMsg `json:"msg"`
	BaseInfo baseInfo          `json:"base_info"`
}

type sendMessageResp struct {
	Ret     int    `json:"ret"`
	Errcode int    `json:"errcode"`
	Errmsg  string `json:"errmsg"`
}

type getConfigReq struct {
	UserID       string   `json:"ilink_user_id"`
	ContextToken string   `json:"context_token,omitempty"`
	BaseInfo     baseInfo `json:"base_info"`
}

type getConfigResp struct {
	Ret          int    `json:"ret"`
	Errcode      int    `json:"errcode"`
	Errmsg       string `json:"errmsg"`
	TypingTicket string `json:"typing_ticket"`
}

type sendTypingReq struct {
	IlinkUserID  string   `json:"ilink_user_id"`
	TypingTicket string   `json:"typing_ticket"`
	Status       int      `json:"status"`
	BaseInfo     baseInfo `json:"base_info"`
}

// --- 消息体 ---

type weixinMessage struct {
	Seq          int64         `json:"seq,omitempty"`
	MessageID    int64         `json:"message_id,omitempty"`
	FromUserID   string        `json:"from_user_id,omitempty"`
	ToUserID     string        `json:"to_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	CreateTimeMs int64         `json:"create_time_ms,omitempty"`
	SessionID    string        `json:"session_id,omitempty"`
	MessageType  int           `json:"message_type,omitempty"`
	MessageState int           `json:"message_state,omitempty"`
	ItemList     []messageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
}

type weixinOutboundMsg struct {
	FromUserID   string        `json:"from_user_id"`
	ToUserID     string        `json:"to_user_id"`
	ClientID     string        `json:"client_id"`
	MessageType  int           `json:"message_type"`
	MessageState int           `json:"message_state"`
	ItemList     []messageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
}

type messageItem struct {
	Type      int        `json:"type,omitempty"`
	TextItem  *textItem  `json:"text_item,omitempty"`
	VoiceItem *voiceItem `json:"voice_item,omitempty"`
	ImageItem *imageItem `json:"image_item,omitempty"`
	FileItem  *fileItem  `json:"file_item,omitempty"`
	VideoItem *videoItem `json:"video_item,omitempty"`
}

type textItem struct {
	Text string `json:"text,omitempty"`
}

type voiceItem struct {
	Text string `json:"text,omitempty"`
}

type imageItem struct {
	AESKeyHex string `json:"aeskey,omitempty"`
}

type fileItem struct {
	FileName string `json:"file_name,omitempty"`
}

type videoItem struct{}

// --- IncomingMessage 统一的入站消息 ---

type IncomingMessage struct {
	FromUserID   string
	Content      string
	ContextToken string
	MessageID    string
}
