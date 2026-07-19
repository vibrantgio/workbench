package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/reactivego/rx"
	"github.com/vibrantgio/mvu"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
)

func LoadConfig(filename string, initial Config) mvu.Command {
	loader := rx.Defer(func() rx.Observable[any] {
		if file, err := os.Open(filename); err == nil {
			defer file.Close()
			decoder := json.NewDecoder(file)
			var cfg Config
			if err = decoder.Decode(&cfg); err == nil {
				return rx.Of[any](cfg)
			}
		}
		return rx.Of[any](initial)
	})
	return mvu.Command{Observable: loader}
}

func SaveConfig(filename string, config Config) mvu.Command {
	return mvu.Command{Observable: rx.Create(func(index int) (Next any, Err error, Done bool) {
		file, err := os.Create(filename)
		if err != nil {
			return nil, err, true
		}
		defer file.Close()
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(config)
		if err != nil {
			return nil, err, true
		}
		return nil, nil, true
	})}
}

// clientFor builds a Responses-dialect client for a provider: its API key,
// and its base URL when set (the SDK appends /responses and /models under
// it, so the catalogue's bases work verbatim for xAI, Groq and
// OpenRouter); an empty BaseURL keeps the OpenAI default.
func clientFor(provider Provider) openai.Client {
	opts := []option.RequestOption{option.WithAPIKey(provider.APIKey)}
	if provider.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(provider.BaseURL))
	}
	return openai.NewClient(opts...)
}

// errText compresses an SDK error to one status-line-sized sentence — the
// SDK's Error() dumps the whole request and response.
func errText(err error) string {
	var apierr *openai.Error
	if errors.As(err, &apierr) {
		msg := apierr.Message
		if msg == "" {
			msg = http.StatusText(apierr.StatusCode)
		}
		return fmt.Sprintf("HTTP %d: %s", apierr.StatusCode, msg)
	}
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// nonChatMarkers flags /models entries that cannot serve chat completions.
// Both OpenAI and xAI list their whole catalogue — speech, image, video,
// embedding and moderation models included — with no protocol-level way to
// tell chat models apart (the discriminating fields are provider-specific),
// so the picker filters on these id substrings instead. Conservative on
// purpose: an odd chat model slipping through beats hiding a real one.
var nonChatMarkers = []string{
	"embed", "whisper", "tts", "dall-e", "moderation",
	"transcribe", "diarize", "imagine", "-image", "-video",
	"audio", "realtime", "babbage", "davinci",
}

// IsChatModel reports whether a /models id looks usable for chat
// completions (see nonChatMarkers).
func IsChatModel(id string) bool {
	lower := strings.ToLower(id)
	for _, marker := range nonChatMarkers {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

// FetchModels lists the provider's /models and reports the chat-capable
// ids (sorted) or the error, tagged with the provider NAME — results are
// matched by name, so a provider renamed mid-flight simply drops the
// stale result.
func FetchModels(provider Provider) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		client := clientFor(provider)
		page, err := client.Models.List(ctx)
		if err != nil {
			return ModelsFetched{Provider: provider.Name, Err: errText(err)}, nil
		}
		var ids []string
		for page != nil {
			for _, m := range page.Data {
				if IsChatModel(m.ID) {
					ids = append(ids, m.ID)
				}
			}
			if page, err = page.GetNextPage(); err != nil {
				break
			}
		}
		slices.Sort(ids)
		return ModelsFetched{Provider: provider.Name, Models: ids}, nil
	})
}

// wireLine is one line of the wire log: the request summary, a raw
// provider event, the transport error, or the end-of-stream marker.
type wireLine struct {
	Time      time.Time       `json:"time"`
	Kind      string          `json:"kind"` // request | event | error | done
	Stream    int             `json:"stream"`
	Chat      string          `json:"chat,omitempty"`
	Provider  string          `json:"provider,omitempty"`
	Model     string          `json:"model,omitempty"`
	Messages  int             `json:"messages,omitempty"`
	WebSearch bool            `json:"web_search,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// wireLog appends raw exchange records to logs/wire-YYYY-MM-DD.jsonl, the
// forensic ground truth when an exchange goes wrong. A nil receiver (the
// log could not be opened) drops writes silently — logging must never
// break the exchange it records.
type wireLog struct{ file *os.File }

func openWireLog(dir string) *wireLog {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	name := filepath.Join(dir, "wire-"+time.Now().Format("2006-01-02")+".jsonl")
	file, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil
	}
	return &wireLog{file: file}
}

func (w *wireLog) line(l wireLine) {
	if w == nil {
		return
	}
	l.Time = time.Now()
	if data, err := json.Marshal(l); err == nil {
		w.file.Write(append(data, '\n'))
	}
}

func (w *wireLog) close() {
	if w != nil {
		w.file.Close()
	}
}

// urlCitation extracts a url_citation annotation's fields; other
// annotation kinds report ok=false.
func urlCitation(annotation any) (url, title string, ok bool) {
	m, isMap := annotation.(map[string]any)
	if !isMap {
		return "", "", false
	}
	if t, _ := m["type"].(string); t != "url_citation" {
		return "", "", false
	}
	url, _ = m["url"].(string)
	title, _ = m["title"].(string)
	return url, title, url != ""
}

// RequestResponse streams a Responses API exchange for the given history
// from the given provider and model, translating the semantic events into
// stream-tagged reducer messages: text deltas, web-search status and
// citations, and an explicit completed/failed/done tail so no ending —
// however abnormal — can pass silently. Every raw event is appended to
// the wire log. Error/status rows in the history are view-only and are
// not sent to the model.
func RequestResponse(id int, provider Provider, modelID string, hist []Message, logdir, chat string) mvu.Command {
	messages := slices.Clone(hist)
	return mvu.Command{Observable: rx.Defer(func() rx.Observable[any] {
		ctx := context.Background()
		input := responses.ResponseInputParam{}
		for _, m := range messages {
			switch m.Role {
			case RoleUser:
				input = append(input, responses.ResponseInputItemParamOfMessage(m.Content, responses.EasyInputMessageRoleUser))
			case RoleAssistant:
				input = append(input, responses.ResponseInputItemParamOfMessage(m.Content, responses.EasyInputMessageRoleAssistant))
			}
		}
		params := responses.ResponseNewParams{
			Model: modelID,
			Input: responses.ResponseNewParamsInputUnion{OfInputItemList: input},
		}
		if provider.WebSearch {
			params.Tools = []responses.ToolUnionParam{responses.ToolParamOfWebSearch(responses.WebSearchToolTypeWebSearch)}
		}
		wire := openWireLog(logdir)
		wire.line(wireLine{Kind: "request", Stream: id, Chat: chat, Provider: provider.Name, Model: modelID, Messages: len(messages), WebSearch: provider.WebSearch})
		client := clientFor(provider)
		stream := client.Responses.NewStreaming(ctx, params)
		failedEmitted, doneEmitted := false, false
		return rx.Create(func(index int) (Next any, Err error, Done bool) {
			for !doneEmitted && stream.Next() {
				ev := stream.Current()
				raw := ev.RawJSON()
				if raw == "" {
					raw = "null"
				}
				wire.line(wireLine{Kind: "event", Stream: id, Event: json.RawMessage(raw)})
				switch ev.Type {
				case "response.output_text.delta":
					return AssistantDelta{Stream: id, Text: ev.Delta}, nil, false
				case "response.output_item.added":
					if ev.Item.Type == "web_search_call" {
						return ToolStatus{Stream: id, Status: "Searching the web…"}, nil, false
					}
				case "response.web_search_call.searching":
					return ToolStatus{Stream: id, Status: "Searching the web…"}, nil, false
				case "response.web_search_call.completed":
					return ToolStatus{Stream: id, Status: ""}, nil, false
				case "response.output_text.annotation.added":
					if url, title, ok := urlCitation(ev.Annotation); ok {
						return CitationAdded{Stream: id, URL: url, Title: title}, nil, false
					}
				case "response.completed":
					return StreamCompleted{Stream: id}, nil, false
				case "response.failed":
					failedEmitted = true
					msg := ev.Response.Error.Message
					if msg == "" {
						msg = "the provider reported a failed response"
					}
					return StreamFailed{Stream: id, Err: msg}, nil, false
				case "response.incomplete":
					failedEmitted = true
					msg := "response incomplete"
					if reason := ev.Response.IncompleteDetails.Reason; reason != "" {
						msg += ": " + string(reason)
					}
					return StreamFailed{Stream: id, Err: msg}, nil, false
				case "error":
					failedEmitted = true
					msg := ev.Message
					if msg == "" {
						msg = "provider error"
					}
					return StreamFailed{Stream: id, Err: msg}, nil, false
				}
				// Everything else (created, in_progress, item.done, …) is
				// wire-log-only; keep draining.
			}
			// Transport drained: report a dying connection or failed
			// request once, then the unconditional cleanup marker.
			if err := stream.Err(); err != nil && !failedEmitted {
				failedEmitted = true
				wire.line(wireLine{Kind: "error", Stream: id, Error: errText(err)})
				return StreamFailed{Stream: id, Err: errText(err)}, nil, false
			}
			if !doneEmitted {
				doneEmitted = true
				wire.line(wireLine{Kind: "done", Stream: id})
				wire.close()
				stream.Close()
				return StreamDone{Stream: id}, nil, false
			}
			return nil, nil, true
		})
	})}
}

// ParseChatFile replays any of the three chat file formats into memory:
// the append-only JSONL event log, the legacy wrapped object, and the
// legacy bare history array. A single JSON document with no event type is
// the legacy object (events always carry one); anything else decodes as a
// stream of ChatEvents whose unknown types are skipped.
func ParseChatFile(data []byte) (ChatFile, error) {
	var cf ChatFile
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return cf, nil
	}
	if trimmed[0] == '[' {
		return cf, json.Unmarshal(trimmed, &cf.History)
	}
	var probe struct {
		Type     string `json:"type"`
		Provider string
		Model    string
		History  []Message
	}
	if err := json.Unmarshal(trimmed, &probe); err == nil && probe.Type == "" {
		return ChatFile{Provider: probe.Provider, Model: probe.Model, History: probe.History}, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	for {
		var e ChatEvent
		if err := decoder.Decode(&e); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return cf, err
		}
		switch e.Type {
		case "meta":
			cf.Provider, cf.Model = e.Provider, e.Model
		case "user":
			cf.History = append(cf.History, Message{Role: RoleUser, Content: e.Text})
		case "assistant":
			cf.History = append(cf.History, Message{Role: RoleAssistant, Content: e.Text, Citations: e.Citations})
		case "error":
			cf.History = append(cf.History, Message{Role: RoleError, Content: e.Error})
		}
	}
	return cf, nil
}

// LoadHist reads a chat's history; the result is tagged with the chat name
// so a slow read can never be applied to a different conversation.
func LoadHist(chat, filename string) mvu.Command {
	return mvu.Command{Observable: rx.Create(func(index int) (Next any, Err error, Done bool) {
		if index == 0 {
			data, err := os.ReadFile(filename)
			if err != nil {
				return nil, err, true
			}
			cf, err := ParseChatFile(data)
			if err != nil {
				return nil, err, true
			}
			return HistLoaded{Chat: chat, Provider: cf.Provider, Model: cf.Model, History: cf.History}, nil, false
		}
		return nil, nil, true
	})}
}

// AppendChatEvent appends one timestamped event line to a chat's JSONL
// history file, creating it if needed. It emits no message; the model was
// already reduced when the command was issued.
func AppendChatEvent(filename string, event ChatEvent) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		event.Time = time.Now()
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return nil, json.NewEncoder(file).Encode(event)
	})
}

// CreateChat touches a fresh chat's (empty) history file so renames and
// deletes have a file to move from the moment the chat exists.
func CreateChat(filename string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, err
		}
		return nil, file.Close()
	})
}

// MigrateChats converts pre-JSONL chat files (*.json) to the append-only
// *.jsonl format, parking the originals under chats/.migrated (directories
// are invisible to the chat list, and the copies remain as backups). Runs
// once at startup after RestoreTrash, so trash leftovers convert too.
// Events are stamped with the source file's modtime — the legacy formats
// carried no timestamps. A file that fails to parse is left untouched: it
// never loaded before either, and converting could destroy evidence.
func MigrateChats(chatdir string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		entries, err := os.ReadDir(chatdir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		park := func(name string) error {
			if err := os.MkdirAll(filepath.Join(chatdir, ".migrated"), 0o755); err != nil {
				return err
			}
			return os.Rename(filepath.Join(chatdir, name), filepath.Join(chatdir, ".migrated", name))
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".json") {
				continue
			}
			target := filepath.Join(chatdir, strings.TrimSuffix(name, ".json")+".jsonl")
			if _, err := os.Stat(target); err == nil {
				// A previous run was interrupted between write and park.
				if err := park(name); err != nil {
					return nil, err
				}
				continue
			}
			data, err := os.ReadFile(filepath.Join(chatdir, name))
			if err != nil {
				return nil, err
			}
			cf, err := ParseChatFile(data)
			if err != nil {
				continue
			}
			stamp := time.Now()
			if info, err := entry.Info(); err == nil {
				stamp = info.ModTime()
			}
			var buf bytes.Buffer
			encoder := json.NewEncoder(&buf)
			if cf.Provider != "" || cf.Model != "" {
				encoder.Encode(ChatEvent{Time: stamp, Type: "meta", Provider: cf.Provider, Model: cf.Model})
			}
			for _, m := range cf.History {
				if m.Role != RoleUser && m.Role != RoleAssistant {
					continue
				}
				encoder.Encode(ChatEvent{Time: stamp, Type: m.Role, Text: m.Content, Citations: m.Citations})
			}
			if err := os.WriteFile(target, buf.Bytes(), 0o644); err != nil {
				return nil, err
			}
			if err := park(name); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
}

// TrashHist moves a chat's history file into the trash directory, where it
// stays undoable. It emits no message; the model was already reduced.
func TrashHist(filename, trashname string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		if err := os.MkdirAll(filepath.Dir(trashname), 0o755); err != nil {
			return nil, err
		}
		return nil, os.Rename(filename, trashname)
	})
}

// RestoreTrash moves every file left in the trash back into the chats
// directory — deletes not undone before the previous quit reappear rather
// than silently vanishing. Runs before LoadConfig at startup. A name that
// meanwhile exists again keeps the live file; the trash copy is dropped.
func RestoreTrash(trashdir, chatdir string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		entries, err := os.ReadDir(trashdir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			target := filepath.Join(chatdir, entry.Name())
			if _, err := os.Stat(target); err == nil {
				_ = os.Remove(filepath.Join(trashdir, entry.Name()))
				continue
			}
			if err := os.Rename(filepath.Join(trashdir, entry.Name()), target); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
}

// RenameHist moves a chat's history file to its new name. It emits no
// message; the model was already reduced when the command was issued.
func RenameHist(oldname, newname string) mvu.Command {
	return mvu.Do(func() (mvu.Message, error) {
		return nil, os.Rename(oldname, newname)
	})
}

// UndoWindow is how long the undo bar stays visible. It is display-only:
// Cmd/Ctrl-Z keeps working for the whole session (the file sits in the
// trash), the bar just stops advertising it.
const UndoWindow = 15 * time.Second

// ExpireDelete hides a delete's undo bar after the delay. The generation
// guards against the timer of a delete whose bar was replaced or dismissed
// in the meantime. rx.Timer (not time.Sleep) keeps the command cancellable,
// so quitting the app mid-window does not block the runner's teardown.
func ExpireDelete(gen int, after time.Duration) mvu.Command {
	return mvu.Command{Observable: rx.Map(rx.Timer[int](after), func(int) any {
		return ConfirmDelete{Gen: gen}
	})}
}

// EditSettleDelay is the quiet period after a key/URL keystroke before the
// key is checked with a /models fetch — long enough not to fire while
// typing, short enough to feel immediate after a paste.
const EditSettleDelay = 600 * time.Millisecond

// SettleProviderEdit emits ProviderEditSettled after the debounce delay.
// Every keystroke arms a new timer under a fresh generation; the reducer
// ignores all but the latest. rx.Timer (not time.Sleep) keeps the command
// cancellable on app teardown.
func SettleProviderEdit(gen int, provider string, after time.Duration) mvu.Command {
	return mvu.Command{Observable: rx.Map(rx.Timer[int](after), func(int) any {
		return ProviderEditSettled{Gen: gen, Provider: provider}
	})}
}

func LoadChatList(directory string) mvu.Command {
	chats := rx.Scan(Directory(directory), []fs.DirEntry(nil), func(acc []fs.DirEntry, entry fs.DirEntry) []fs.DirEntry {
		return append(acc, entry)
	})
	return mvu.Command{Observable: rx.Map(chats, func(entries []fs.DirEntry) any {
		var names ChatList
		for _, entry := range entries {
			// Skip directories — notably .trash (undoable deletes) and
			// .migrated (pre-JSONL backups), which are not live chats.
			if entry.IsDir() {
				continue
			}
			names = append(names, entry.Name())
		}
		return names
	})}
}
