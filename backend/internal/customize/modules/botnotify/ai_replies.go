package botnotify

import "strings"

// Fixed, non-generated replies for every path that does not produce a model
// answer. Each one tells the user what to do next instead of a bare refusal.

// supportHint names the configured human support channel.
func supportHint(dto PublicDTO) string {
	var parts []string
	if v := strings.TrimSpace(dto.ContactInfo); v != "" {
		parts = append(parts, v)
	}
	if v := strings.TrimSpace(dto.AfterSalesGroup); v != "" {
		parts = append(parts, "售后群 "+v)
	}
	if v := strings.TrimSpace(dto.CustomerServiceLink); v != "" {
		parts = append(parts, v)
	}
	if len(parts) == 0 {
		return "可以通过网站页面上的「联系客服」入口联系人工客服。"
	}
	return "人工客服：" + strings.Join(parts, "；") + "。"
}

// BusyReply covers transient provider failures (rate limits, timeouts, 5xx).
func BusyReply(dto PublicDTO) string {
	return "抱歉，AI 客服暂时繁忙，请稍后再问一次。" + supportHint(dto)
}

// UnavailableReply covers failures that need an administrator (key disabled,
// balance exhausted, model unavailable); repeating the question will not help.
func UnavailableReply(dto PublicDTO) string {
	return "抱歉，AI 客服暂时无法使用。" + supportHint(dto)
}

// OutputBlockedReply replaces an answer that failed the output checks.
func OutputBlockedReply(dto PublicDTO) string {
	return "抱歉，这个问题我暂时无法给出可靠的回答，请换个问法再试。" + supportHint(dto)
}

// DailyLimitReply is sent once the bot's daily answer budget is used up.
func DailyLimitReply(dto PublicDTO) string {
	return "今天的 AI 客服咨询量已达上限，请明天再来。" + supportHint(dto)
}

// AttachmentReply answers messages the bot cannot read at all.
func AttachmentReply(kind string, imagesAllowed bool) string {
	switch kind {
	case "image":
		if !imagesAllowed {
			return "暂时不支持识别图片，请用文字描述你的问题，报错信息可以直接复制发给我。"
		}
		return "这张图片太大或格式不支持，请截取报错部分重新发送，或直接把报错文字复制给我。"
	case "voice":
		return "暂时听不了语音，请用文字描述你的问题。"
	case "video":
		return "暂时看不了视频或动图，请截图或用文字描述你的问题。"
	}
	return "暂时无法读取这类文件，请截图或用文字描述你的问题。"
}
