package gatewaylogarchive

// Archive inspection is deliberately separate from protocol forwarding. Native
// bodies stay untouched; derived responses are only built by the export worker,
// and absent signatures/calls are never synthesized.
import (
	"bufio"
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
)

type toolCallInfo struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Valid bool   `json:"valid"`
}

type exchangeAnalysis struct {
	Protocol                 string          `json:"protocol"`
	Normalized               json.RawMessage `json:"normalized_response,omitempty"`
	AssistantTurns           int             `json:"assistant_turns"`
	RequestAssistantMessages int             `json:"request_assistant_messages"`
	ThinkingBlocks           int             `json:"thinking_blocks"`
	Signatures               int             `json:"signatures"`
	ToolCalls                []toolCallInfo  `json:"tool_calls,omitempty"`
	ToolResults              []string        `json:"tool_result_ids,omitempty"`
	ResponseID               string          `json:"response_id,omitempty"`
	PreviousResponseID       string          `json:"previous_response_id,omitempty"`
	StreamComplete           bool            `json:"stream_complete"`
	CaptureTruncated         bool            `json:"capture_truncated"`
	Warnings                 []string        `json:"warnings,omitempty"`
}

func (a *exchangeAnalysis) warn(s string) {
	for _, old := range a.Warnings {
		if old == s {
			return
		}
	}
	a.Warnings = append(a.Warnings, s)
}

type rawObject map[string]json.RawMessage

func objectOf(raw string) rawObject {
	v := rawObject{}
	_ = json.Unmarshal([]byte(raw), &v)
	if v == nil {
		v = rawObject{}
	}
	return v
}
func rawJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// Token-by-token streams can contain many tiny deltas. Accumulate once rather
// than repeatedly decoding and copying the entire text after every token.
type stringDeltas map[string]*strings.Builder

func (d stringDeltas) append(obj rawObject, key, value string) {
	b := d[key]
	if b == nil {
		b = &strings.Builder{}
		var initial string
		_ = json.Unmarshal(obj[key], &initial)
		b.WriteString(initial)
		d[key] = b
	}
	b.WriteString(value)
}
func (d stringDeltas) finish(obj rawObject) {
	for key, b := range d {
		obj[key] = rawJSON(b.String())
	}
}
func objectKeys[T any](m map[int]T) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// forSSEData has no Scanner token limit. Multi-line data fields are joined per
// SSE rules. Raw events are retained in the enclosing archive regardless.
func forSSEData(text string, visit func(string)) {
	r := bufio.NewReader(strings.NewReader(text))
	var data []string
	emit := func() {
		if len(data) > 0 {
			visit(strings.Join(data, "\n"))
			data = nil
		}
	}
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			emit()
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(line[5:], " "))
		}
		if err != nil {
			emit()
			return
		}
	}
}

type anthropicBlock struct {
	obj     rawObject
	text    stringDeltas
	input   strings.Builder
	stopped bool
}
type chatChoice struct {
	message  rawObject
	text     stringDeltas
	tools    map[int]rawObject
	toolText map[int]stringDeltas
	finish   string
}
type responseItemDeltas struct {
	fields  stringDeltas
	content map[int]stringDeltas
	summary map[int]stringDeltas
}
type responseAssembler struct {
	analysis   *exchangeAnalysis
	message    rawObject
	blocks     map[int]*anthropicBlock
	choices    map[int]*chatChoice
	items      map[int]rawObject
	itemDeltas map[int]*responseItemDeltas
	gemini     map[int][]json.RawMessage
	done       bool
}

func analyzeExchange(raw []byte) exchangeAnalysis {
	a := exchangeAnalysis{Protocol: "unknown"}
	root := gjson.ParseBytes(raw)
	a.CaptureTruncated = root.Get("request_truncated").Bool() || root.Get("response_truncated").Bool()
	if a.CaptureTruncated {
		a.warn("capture_truncated")
	}
	if root.Get("response_content_encoding").Exists() {
		a.warn("response_not_decoded")
	}
	if root.Get("request_encoding").Exists() || root.Get("response_encoding").Exists() {
		a.warn("body_not_text")
	}
	request := root.Get("request")
	a.PreviousResponseID = request.Get("previous_response_id").String()
	inspectRequest(request, &a)
	s := &responseAssembler{analysis: &a, blocks: map[int]*anthropicBlock{}, choices: map[int]*chatChoice{}, items: map[int]rawObject{}, itemDeltas: map[int]*responseItemDeltas{}, gemini: map[int][]json.RawMessage{}}
	response := root.Get("response")
	stream := root.Get("stream").Bool() || root.Get("transport").String() == "websocket"
	if root.Get("transport").String() == "websocket" && response.IsArray() {
		for _, event := range response.Array() {
			if event.Get("direction").String() != "client" {
				s.event(event.Raw)
			}
		}
	} else if response.Type == gjson.String && stream {
		forSSEData(response.String(), s.event)
	} else if response.IsObject() {
		a.Normalized = json.RawMessage(response.Raw)
		s.done = true
	} else {
		a.warn("response_unrecognized")
	}
	if stream {
		s.finish()
	}
	a.StreamComplete = s.done && !a.CaptureTruncated
	if stream && !s.done {
		a.warn("stream_terminal_missing")
	}
	if len(a.Normalized) > 0 {
		inspectResponse(gjson.ParseBytes(a.Normalized), &a)
	} else {
		a.warn("response_unrecognized")
	}
	if root.Get("request").Type == gjson.String {
		a.warn("request_not_structured")
	}
	if a.ThinkingBlocks > 0 && a.Signatures == 0 {
		a.warn("reasoning_signature_not_observed")
	}
	for _, call := range a.ToolCalls {
		if !call.Valid {
			a.warn("invalid_tool_call")
		}
	}
	if status := root.Get("status").Int(); status >= 400 {
		a.warn("request_failed")
	}
	for _, warning := range a.Warnings {
		switch warning {
		case "malformed_stream_event", "content_block_start_missing", "message_start_missing", "content_block_stop_missing", "output_item_start_missing", "content_part_start_missing", "invalid_content_index", "invalid_summary_index", "response_unrecognized", "upstream_stream_error", "response_not_decoded", "body_not_text":
			a.StreamComplete = false
		}
	}
	return a
}

func (s *responseAssembler) event(raw string) {
	if strings.TrimSpace(raw) == "[DONE]" {
		s.done = true
		return
	}
	if !gjson.Valid(raw) {
		s.analysis.warn("malformed_stream_event")
		return
	}
	e := gjson.Parse(raw)
	typ := e.Get("type").String()
	switch {
	case typ == "message_start":
		s.analysis.Protocol = "anthropic"
		s.message = objectOf(e.Get("message").Raw)
	case typ == "content_block_start":
		s.analysis.Protocol = "anthropic"
		s.blocks[int(e.Get("index").Int())] = &anthropicBlock{obj: objectOf(e.Get("content_block").Raw), text: stringDeltas{}}
	case typ == "content_block_delta":
		b := s.blocks[int(e.Get("index").Int())]
		if b == nil {
			s.analysis.warn("content_block_start_missing")
			return
		}
		d := e.Get("delta")
		switch d.Get("type").String() {
		case "text_delta":
			b.text.append(b.obj, "text", d.Get("text").String())
		case "thinking_delta":
			b.text.append(b.obj, "thinking", d.Get("thinking").String())
		case "signature_delta":
			b.text.append(b.obj, "signature", d.Get("signature").String())
		case "input_json_delta":
			b.input.WriteString(d.Get("partial_json").String())
		}
	case typ == "content_block_stop":
		if b := s.blocks[int(e.Get("index").Int())]; b != nil {
			b.stopped = true
		}
	case typ == "message_delta":
		if s.message == nil {
			s.message = rawObject{}
		}
		for key, value := range objectOf(e.Get("delta").Raw) {
			s.message[key] = value
		}
		if e.Get("usage").Exists() {
			s.message["usage"] = json.RawMessage(e.Get("usage").Raw)
		}
	case typ == "message_stop":
		s.done = true
	case typ == "error":
		s.analysis.warn("upstream_stream_error")
	case strings.HasPrefix(typ, "response."):
		s.analysis.Protocol = "responses"
		s.responsesEvent(e, typ)
	case e.Get("choices").IsArray():
		s.analysis.Protocol = "chat_completions"
		for _, choice := range e.Get("choices").Array() {
			i := int(choice.Get("index").Int())
			c := s.choices[i]
			if c == nil {
				c = &chatChoice{message: rawObject{"role": rawJSON("assistant")}, text: stringDeltas{}, tools: map[int]rawObject{}, toolText: map[int]stringDeltas{}}
				s.choices[i] = c
			}
			if message := choice.Get("message"); message.IsObject() {
				c.message = objectOf(message.Raw)
			}
			d := choice.Get("delta")
			for _, key := range []string{"content", "reasoning_content", "reasoning", "refusal"} {
				if v := d.Get(key); v.Type == gjson.String {
					c.text.append(c.message, key, v.String())
				}
			}
			for _, tool := range d.Get("tool_calls").Array() {
				idx := int(tool.Get("index").Int())
				t := c.tools[idx]
				if t == nil {
					t = rawObject{}
					c.tools[idx] = t
					c.toolText[idx] = stringDeltas{}
				}
				for _, key := range []string{"id", "type"} {
					if v := tool.Get(key); v.Exists() {
						t[key] = json.RawMessage(v.Raw)
					}
				}
				f := objectOf(string(t["function"]))
				for _, key := range []string{"name", "arguments"} {
					if v := tool.Get("function." + key); v.Exists() {
						c.toolText[idx].append(f, key, v.String())
					}
				}
				t["function"] = rawJSON(f)
			}
			if v := choice.Get("finish_reason"); v.Type == gjson.String && v.String() != "" {
				c.finish = v.String()
			}
		}
		if s.message == nil {
			s.message = rawObject{}
		}
		for _, key := range []string{"id", "model", "created"} {
			if v := e.Get(key); v.Exists() {
				s.message[key] = json.RawMessage(v.Raw)
			}
		}
		if e.Get("usage").IsObject() {
			s.message["usage"] = json.RawMessage(e.Get("usage").Raw)
		}
	case e.Get("candidates").IsArray():
		s.analysis.Protocol = "gemini"
		for _, c := range e.Get("candidates").Array() {
			i := int(c.Get("index").Int())
			for _, p := range c.Get("content.parts").Array() {
				s.gemini[i] = append(s.gemini[i], json.RawMessage(p.Raw))
			}
			if finish := c.Get("finishReason").String(); finish != "" {
				s.done = true
				if finish != "STOP" {
					s.analysis.warn("finish_reason:" + finish)
				}
			}
		}
	}
}

func (s *responseAssembler) responsesEvent(e gjson.Result, typ string) {
	i := int(e.Get("output_index").Int())
	deltas := s.itemDeltas[i]
	if deltas == nil {
		deltas = &responseItemDeltas{fields: stringDeltas{}, content: map[int]stringDeltas{}, summary: map[int]stringDeltas{}}
		s.itemDeltas[i] = deltas
	}
	switch typ {
	case "response.created", "response.in_progress":
		if s.message == nil {
			s.message = objectOf(e.Get("response").Raw)
		}
	case "response.output_item.added", "response.output_item.done":
		s.items[i] = objectOf(e.Get("item").Raw)
		delete(s.itemDeltas, i)
	case "response.function_call_arguments.delta":
		if item := s.items[i]; item != nil {
			deltas.fields.append(item, "arguments", e.Get("delta").String())
		} else {
			s.analysis.warn("output_item_start_missing")
		}
	case "response.function_call_arguments.done":
		if item := s.items[i]; item != nil {
			item["arguments"] = json.RawMessage(e.Get("arguments").Raw)
			delete(deltas.fields, "arguments")
		}
	case "response.content_part.added", "response.content_part.done":
		item := s.items[i]
		if item == nil {
			s.analysis.warn("output_item_start_missing")
			return
		}
		var content []json.RawMessage
		_ = json.Unmarshal(item["content"], &content)
		idx := int(e.Get("content_index").Int())
		if idx < 0 || idx > 10000 {
			s.analysis.warn("invalid_content_index")
			return
		}
		for len(content) <= idx {
			content = append(content, json.RawMessage(`{}`))
		}
		content[idx] = json.RawMessage(e.Get("part").Raw)
		delete(deltas.content, idx)
		item["content"] = rawJSON(content)
	case "response.output_text.delta":
		item := s.items[i]
		if item == nil {
			s.analysis.warn("output_item_start_missing")
			return
		}
		var content []json.RawMessage
		_ = json.Unmarshal(item["content"], &content)
		idx := int(e.Get("content_index").Int())
		if idx < 0 || idx >= len(content) {
			s.analysis.warn("content_part_start_missing")
			return
		}
		part := objectOf(string(content[idx]))
		if deltas.content[idx] == nil {
			deltas.content[idx] = stringDeltas{}
		}
		deltas.content[idx].append(part, "text", e.Get("delta").String())
	case "response.completed", "response.failed", "response.incomplete", "response.cancelled":
		response := e.Get("response")
		if response.IsObject() {
			if s.message == nil {
				s.message = rawObject{}
			}
			for key, value := range objectOf(response.Raw) {
				s.message[key] = value
			}
		}
		s.done = typ == "response.completed"
		if !s.done {
			s.analysis.warn(typ)
		}
	case "response.reasoning_summary_part.added", "response.reasoning_summary_part.done", "response.reasoning_summary_text.delta", "response.reasoning_summary_text.done":
		item := s.items[i]
		if item == nil {
			s.analysis.warn("output_item_start_missing")
			return
		}
		var parts []json.RawMessage
		_ = json.Unmarshal(item["summary"], &parts)
		idx := int(e.Get("summary_index").Int())
		if idx < 0 || idx > 10000 {
			s.analysis.warn("invalid_summary_index")
			return
		}
		for len(parts) <= idx {
			parts = append(parts, json.RawMessage(`{}`))
		}
		if typ == "response.reasoning_summary_part.added" || typ == "response.reasoning_summary_part.done" {
			parts[idx] = json.RawMessage(e.Get("part").Raw)
			delete(deltas.summary, idx)
		} else {
			part := objectOf(string(parts[idx]))
			if typ == "response.reasoning_summary_text.delta" {
				if deltas.summary[idx] == nil {
					deltas.summary[idx] = stringDeltas{}
				}
				deltas.summary[idx].append(part, "text", e.Get("delta").String())
			} else {
				part["text"] = json.RawMessage(e.Get("text").Raw)
				delete(deltas.summary, idx)
			}
			parts[idx] = rawJSON(part)
		}
		item["summary"] = rawJSON(parts)
	}
}

func (s *responseAssembler) finish() {
	switch s.analysis.Protocol {
	case "anthropic":
		if s.message == nil {
			s.message = rawObject{}
			s.analysis.warn("message_start_missing")
		}
		var content []rawObject
		if len(s.blocks) == 0 {
			_ = json.Unmarshal(s.message["content"], &content)
		}
		for _, i := range objectKeys(s.blocks) {
			b := s.blocks[i]
			b.text.finish(b.obj)
			if !b.stopped {
				s.analysis.warn("content_block_stop_missing")
			}
			if b.input.Len() > 0 {
				if gjson.Valid(b.input.String()) {
					b.obj["input"] = json.RawMessage(b.input.String())
				} else {
					b.obj["input"] = rawJSON(b.input.String())
					s.analysis.warn("invalid_tool_arguments")
				}
			}
			content = append(content, b.obj)
		}
		s.message["content"] = rawJSON(content)
	case "chat_completions":
		var choices []any
		allFinished := len(s.choices) > 0
		for _, i := range objectKeys(s.choices) {
			c := s.choices[i]
			c.text.finish(c.message)
			if c.finish == "" {
				allFinished = false
			}
			var tools []rawObject
			for _, idx := range objectKeys(c.tools) {
				f := objectOf(string(c.tools[idx]["function"]))
				c.toolText[idx].finish(f)
				c.tools[idx]["function"] = rawJSON(f)
				tools = append(tools, c.tools[idx])
			}
			if len(tools) > 0 {
				c.message["tool_calls"] = rawJSON(tools)
			}
			choices = append(choices, map[string]any{"index": i, "message": c.message, "finish_reason": c.finish})
		}
		if s.message == nil {
			s.message = rawObject{}
		}
		s.message["choices"] = rawJSON(choices)
		s.done = s.done || allFinished
	case "responses":
		if s.message == nil {
			s.message = rawObject{}
		}
		if !gjson.ParseBytes(s.message["output"]).IsArray() || (len(gjson.ParseBytes(s.message["output"]).Array()) == 0 && len(s.items) > 0) {
			var items []rawObject
			for _, i := range objectKeys(s.items) {
				if delta := s.itemDeltas[i]; delta != nil {
					delta.fields.finish(s.items[i])
					for field, pending := range map[string]map[int]stringDeltas{"content": delta.content, "summary": delta.summary} {
						var parts []json.RawMessage
						_ = json.Unmarshal(s.items[i][field], &parts)
						for index, text := range pending {
							if index < len(parts) {
								part := objectOf(string(parts[index]))
								text.finish(part)
								parts[index] = rawJSON(part)
							}
						}
						if len(parts) > 0 {
							s.items[i][field] = rawJSON(parts)
						}
					}
				}
				items = append(items, s.items[i])
			}
			s.message["output"] = rawJSON(items)
		}
	case "gemini":
		var candidates []any
		for _, i := range objectKeys(s.gemini) {
			candidates = append(candidates, map[string]any{"index": i, "content": map[string]any{"role": "model", "parts": s.gemini[i]}})
		}
		s.message = rawObject{"candidates": rawJSON(candidates)}
	}
	if s.message != nil {
		s.analysis.Normalized = rawJSON(s.message)
	}
}

func validArgs(v gjson.Result) bool {
	if v.Type == gjson.String {
		return gjson.Valid(v.String()) && gjson.Parse(v.String()).IsObject()
	}
	return v.IsObject()
}
func inspectContent(content gjson.Result, a *exchangeAnalysis) {
	for _, b := range content.Array() {
		switch b.Get("type").String() {
		case "thinking", "redacted_thinking":
			a.ThinkingBlocks++
			if b.Get("type").String() == "thinking" && b.Get("signature").String() == "" {
				a.warn("thinking_without_signature")
			}
		case "tool_use":
			a.ToolCalls = append(a.ToolCalls, toolCallInfo{ID: b.Get("id").String(), Name: b.Get("name").String(), Valid: b.Get("id").String() != "" && b.Get("name").String() != "" && b.Get("input").IsObject()})
		case "tool_result":
			a.ToolResults = append(a.ToolResults, b.Get("tool_use_id").String())
		}
		if b.Get("signature").String() != "" {
			a.Signatures++
			if strings.Contains(b.Get("thinking").String(), piiMask) {
				a.warn("signed_thinking_content_redacted")
			}
		}
	}
}
func inspectRequest(r gjson.Result, a *exchangeAnalysis) {
	for _, m := range r.Get("messages").Array() {
		if m.Get("role").String() == "assistant" {
			a.RequestAssistantMessages++
		}
		if m.Get("role").String() == "assistant" {
			inspectContent(m.Get("content"), a)
			inspectChatTools(m, a)
		} else {
			for _, b := range m.Get("content").Array() {
				if b.Get("type").String() == "tool_result" {
					a.ToolResults = append(a.ToolResults, b.Get("tool_use_id").String())
				}
				if b.Get("type").String() == "tool_use" {
					a.warn("tool_use_outside_assistant")
				}
			}
		}
		if m.Get("role").String() == "tool" {
			a.ToolResults = append(a.ToolResults, m.Get("tool_call_id").String())
		}
	}
	inspectResponsesItems(r.Get("input"), a, false)
	for _, m := range r.Get("contents").Array() {
		if m.Get("role").String() == "model" {
			a.RequestAssistantMessages++
		}
		inspectGeminiParts(m.Get("parts"), a)
	}
}
func inspectChatTools(m gjson.Result, a *exchangeAnalysis) {
	for _, c := range m.Get("tool_calls").Array() {
		a.ToolCalls = append(a.ToolCalls, toolCallInfo{ID: c.Get("id").String(), Name: c.Get("function.name").String(), Valid: c.Get("id").String() != "" && c.Get("function.name").String() != "" && c.Get("function.arguments").Type == gjson.String && validArgs(c.Get("function.arguments"))})
	}
}
func inspectResponsesItems(items gjson.Result, a *exchangeAnalysis, response bool) {
	for _, item := range items.Array() {
		switch item.Get("type").String() {
		case "message":
			if !response && item.Get("role").String() == "assistant" {
				a.RequestAssistantMessages++
			}
		case "reasoning":
			a.ThinkingBlocks++
			if item.Get("encrypted_content").String() != "" {
				a.Signatures++
			} else {
				a.warn("reasoning_without_encrypted_content")
			}
		case "function_call":
			a.ToolCalls = append(a.ToolCalls, toolCallInfo{ID: item.Get("call_id").String(), Name: item.Get("name").String(), Valid: item.Get("call_id").String() != "" && item.Get("name").String() != "" && item.Get("arguments").Type == gjson.String && validArgs(item.Get("arguments"))})
		case "function_call_output":
			a.ToolResults = append(a.ToolResults, item.Get("call_id").String())
		case "compaction":
			a.warn("context_compaction_observed")
		}
	}
}
func inspectGeminiParts(parts gjson.Result, a *exchangeAnalysis) {
	for _, p := range parts.Array() {
		if p.Get("thought").Bool() {
			a.ThinkingBlocks++
		}
		if p.Get("thoughtSignature").String() != "" {
			a.Signatures++
		}
		if f := p.Get("functionCall"); f.Exists() {
			a.ToolCalls = append(a.ToolCalls, toolCallInfo{ID: f.Get("id").String(), Name: f.Get("name").String(), Valid: f.Get("name").String() != "" && f.Get("args").IsObject()})
		}
		if p.Get("functionResponse").Exists() {
			id := p.Get("functionResponse.id").String()
			if id != "" {
				a.ToolResults = append(a.ToolResults, id)
			} else {
				a.warn("tool_pairing_without_id")
			}
		}
	}
}
func inspectResponse(r gjson.Result, a *exchangeAnalysis) {
	a.ResponseID = r.Get("id").String()
	switch {
	case r.Get("content").IsArray():
		a.Protocol = "anthropic"
		if len(r.Get("content").Array()) > 0 {
			a.AssistantTurns = 1
		}
		inspectContent(r.Get("content"), a)
	case r.Get("choices").IsArray():
		a.Protocol = "chat_completions"
		for _, c := range r.Get("choices").Array() {
			m := c.Get("message")
			if m.Exists() {
				a.AssistantTurns = 1
			}
			inspectChatTools(m, a)
			if m.Get("reasoning_content").String() != "" || m.Get("reasoning").String() != "" {
				a.ThinkingBlocks++
			}
			if c.Get("finish_reason").String() == "length" {
				a.warn("output_token_limit")
			}
		}
	case r.Get("output").IsArray():
		a.Protocol = "responses"
		if len(r.Get("output").Array()) > 0 {
			a.AssistantTurns = 1
		}
		inspectResponsesItems(r.Get("output"), a, true)
		if status := r.Get("status").String(); status != "" && status != "completed" {
			a.warn("response_status:" + status)
		}
	case r.Get("candidates").IsArray():
		a.Protocol = "gemini"
		if len(r.Get("candidates").Array()) > 0 {
			a.AssistantTurns = 1
		}
		for _, c := range r.Get("candidates").Array() {
			inspectGeminiParts(c.Get("content.parts"), a)
			if c.Get("finishReason").String() == "MAX_TOKENS" {
				a.warn("output_token_limit")
			}
		}
	default:
		a.warn("response_unrecognized")
	}
	if r.Get("stop_reason").String() == "max_tokens" {
		a.warn("output_token_limit")
	}
	if r.Get("stop_reason").String() == "pause_turn" {
		a.warn("assistant_paused")
	}
}

// addAnalysis keeps the original exchange tokens and adds a derived field. It
// intentionally does not marshal the raw request/response through float64/maps.
func addAnalysis(raw []byte, a exchangeAnalysis) []byte {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[len(raw)-1] != '}' {
		return raw
	}
	out := make([]byte, 0, len(raw)+512)
	out = append(out, raw[:len(raw)-1]...)
	out = append(out, `,"analysis":`...)
	out = append(out, rawJSON(a)...)
	return append(out, '}')
}
