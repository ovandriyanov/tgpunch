package tgapi

type GetUpdates struct {
	Offset          int                   `json:"offset"`
	Limit           int                   `json:"limit"`
	Timeout         int                   `json:"timeout"`
	AllowedUpdates  []string              `json:"allowed_updates"`
}

type GetMeResponse struct {
	Ok              bool                  `json:"ok"`
	Result          *User                 `json:"result"`
    Description     *string               `json:"description"`
}

type GetUpdatesResponse struct {
	Ok              bool                  `json:"ok"`
	Result          []Update              `json:"result"`
    Description     *string               `json:"description"`
}

type SendMessageResponse struct {
	Ok              bool                  `json:"ok"`
	Result          *Message              `json:"result"`
    Description     *string               `json:"description"`
}

type Update struct {
	Id              int                   `json:"update_id"`
	Message         *Message              `json:"message"`
	ChannelPost     *Message              `json:"channel_post"`
	InlineQuery     *InlineQuery          `json:"inline_query"`
}

type Chat struct {
	Id                  int64              `json:"id"`
	ChatType            string             `json:"chat_type"`
	Title               *string            `json:"title"`
	Username            *string            `json:"username"`
	FirstName           *string            `json:"first_name"`
	LastName            *string            `json:"last_name"`
	Description         *string            `json:"description"`
	InviteLink          *string            `json:"invite_link"`
	PinnedMessage       *Message           `json:"pinned_message"`
	StickerSetName      *string            `json:"sticker_set_name"`
	CanSetStickerSet    *bool              `json:"can_set_sticker_set"`
}

type Message struct {
	Id              int                   `json:"message_id"`
	Chat            Chat                  `json:"chat"`
	Text            *string               `json:"text"`
	Sticker         *Sticker              `json:"sticker"`
}

type Sticker struct {
	FileId          string                `json:"file_id"`
}

type InlineQuery struct {
	Id              string                `json:"id"`
	From            User                  `json:"from"`
	Query           string                `json:"query"`
	Offset          string                `json:"offset"`
}

type User struct {
	Id              int                   `json:"id"`
	IsBot           bool                  `json:"is_bot"`
	FirstName       string                `json:"first_name"`
	LastName        *string               `json:"last_name"`
	UserName        *string               `json:"username"`
	LanguageCode    *string               `json:"language_code"`
}

type InlineQueryResultCachedSticker struct {
	Type            string                `json:"type"`
	Id              string                `json:"id"`
	StickerFileId   string                `json:"sticker_file_id"`
}

type AnswerInlineQuery struct {
	InlineQueryId   string                           `json:"inline_query_id"`
	Results         []InlineQueryResultCachedSticker `json:"results"`
}
