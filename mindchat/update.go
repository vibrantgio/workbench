package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/vibrantgio/mvu"
)

// Update is the MVU update function: it reduces the model and returns the
// command whose messages the runner in main.go feeds back into the loop.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	switch message := message.(type) {

	case Config:
		// A pre-migration config may still point at a .json chat; the file
		// is now its .jsonl conversion.
		last := message.LastChat
		if strings.HasSuffix(last, ".json") {
			last = strings.TrimSuffix(last, ".json") + ".jsonl"
		}
		model.CurrentChat.Name = last
		model.SidebarRatio = message.SidebarRatio
		model.SidebarCollapsed = message.SidebarCollapsed
		model.Providers = message.Providers
		// Model caches persisted before a nonChatMarkers change (or by an
		// older build) are re-filtered on the way in.
		for i, p := range model.Providers {
			model.Providers[i].Models = slices.DeleteFunc(slices.Clone(p.Models), func(id string) bool { return !IsChatModel(id) })
		}
		model.DefaultProvider = message.DefaultProvider
		model.DefaultModel = message.DefaultModel
		// First run (or a config predating providers): seed the catalogue
		// from the OPENAI_API_KEY environment so existing setups keep
		// working; it persists on the next config save.
		if len(model.Providers) == 0 && model.AuthToken != "" {
			model.Providers = []Provider{{Name: "OpenAI", APIKey: model.AuthToken}}
			model.DefaultProvider, model.DefaultModel = "OpenAI", "gpt-5.5"
		}
		commands := []mvu.Command{LoadChatList(model.ChatDir()).Trace("Load Chat List")}
		// LastChat is empty when the last chat was deleted; there is no
		// history to load then.
		if last != "" {
			commands = append(commands, LoadHist(last, model.ChatFile(last)).Trace("Load History"))
		}
		return model, mvu.DoConcurrent(commands...)

	case Prompt:
		command := mvu.DoNothing()
		if message.Content != "" {
			// Prompting with no chat selected (every chat was deleted)
			// starts a fresh one, so the exchange has a file to persist to.
			if model.CurrentChat.Name == "" {
				model.CurrentChat = Chat{Name: FreshChatName(model.TakenNames()), Loaded: true}
				model.ChatList = append(slices.Clone(model.ChatList), model.CurrentChat.Name)
			}
			model.CurrentChat.History = append(model.CurrentChat.History, Message{Role: RoleUser, Content: message.Content})
			// Register the stream so its events stay routable to THIS chat
			// whatever is current when they arrive; it carries the chat's
			// model override so stream saves preserve it in the file.
			model.NextStream++
			streams := cloneStreams(model.Streams)
			streams[model.NextStream] = StreamState{
				Chat:     model.CurrentChat.Name,
				Provider: model.CurrentChat.Provider,
				Model:    model.CurrentChat.Model,
			}
			model.Streams = streams
			// The prompt is persisted BEFORE the request runs, so even an
			// exchange that dies instantly leaves it on disk. The chat's
			// override wins over the global default.
			provider, modelID, _ := model.EffectiveModel()
			command = mvu.DoSequence(
				AppendChatEvent(model.ChatFile(model.CurrentChat.Name), ChatEvent{Type: "user", Text: message.Content}).Trace("Append Prompt"),
				RequestResponse(model.NextStream, provider, modelID, model.CurrentChat.History, model.LogDir(), model.CurrentChat.Name).Trace("Request Response"),
				SaveConfig(model.ConfigFile(), model.Config()).Trace("Save Config"),
			)
		}
		return model, command

	case AssistantDelta:
		s, tracked := model.Streams[message.Stream]
		if !tracked {
			// Untracked stream: its chat was deleted mid-stream. Drop the
			// delta; the connection drains harmlessly.
			return model, mvu.DoNothing()
		}
		hist := streamHist(model, s)
		// The exchange's first delta opens the assistant row; later ones
		// extend it.
		if n := len(hist); n > 0 && hist[n-1].Role == RoleAssistant {
			hist[n-1].Content += message.Text
		} else {
			hist = append(hist, Message{Role: RoleAssistant, Content: message.Text})
		}
		return storeStreamHist(model, message.Stream, s, hist), mvu.DoNothing()

	case ToolStatus:
		s, tracked := model.Streams[message.Stream]
		if !tracked {
			return model, mvu.DoNothing()
		}
		streams := cloneStreams(model.Streams)
		s.Status = message.Status
		streams[message.Stream] = s
		model.Streams = streams
		return model, mvu.DoNothing()

	case CitationAdded:
		s, tracked := model.Streams[message.Stream]
		if !tracked {
			return model, mvu.DoNothing()
		}
		hist := streamHist(model, s)
		if n := len(hist); n == 0 || hist[n-1].Role != RoleAssistant {
			// A citation can precede the first text delta: open the row.
			hist = append(hist, Message{Role: RoleAssistant})
		}
		last := &hist[len(hist)-1]
		if !slices.ContainsFunc(last.Citations, func(c Citation) bool { return c.URL == message.URL }) {
			last.Citations = append(slices.Clone(last.Citations), Citation{URL: message.URL, Title: message.Title})
		}
		return storeStreamHist(model, message.Stream, s, hist), mvu.DoNothing()

	case StreamCompleted:
		s, tracked := model.Streams[message.Stream]
		if !tracked {
			return model, mvu.DoNothing()
		}
		hist := streamHist(model, s)
		streams := cloneStreams(model.Streams)
		delete(streams, message.Stream)
		model.Streams = streams
		// Persist the final assistant row to the stream's OWN chat file —
		// never to whatever chat happens to be current.
		if n := len(hist); n > 0 && hist[n-1].Role == RoleAssistant {
			last := hist[n-1]
			return model, AppendChatEvent(model.ChatFile(s.Chat), ChatEvent{Type: "assistant", Text: last.Content, Citations: last.Citations}).Trace("Append Reply")
		}
		return model, mvu.DoNothing()

	case StreamFailed:
		s, tracked := model.Streams[message.Stream]
		if !tracked {
			return model, mvu.DoNothing()
		}
		// Persist any partial answer, then the error — and show the error
		// in the chat itself, so a failed exchange is never a silent one.
		hist := streamHist(model, s)
		var commands []mvu.Command
		if n := len(hist); n > 0 && hist[n-1].Role == RoleAssistant {
			last := hist[n-1]
			commands = append(commands, AppendChatEvent(model.ChatFile(s.Chat), ChatEvent{Type: "assistant", Text: last.Content, Citations: last.Citations}).Trace("Append Partial Reply"))
		}
		commands = append(commands, AppendChatEvent(model.ChatFile(s.Chat), ChatEvent{Type: "error", Error: message.Err}).Trace("Append Error"))
		hist = append(hist, Message{Role: RoleError, Content: message.Err})
		model = storeStreamHist(model, message.Stream, s, hist)
		streams := cloneStreams(model.Streams)
		delete(streams, message.Stream)
		model.Streams = streams
		return model, mvu.DoSequence(commands...)

	case StreamDone:
		// After StreamCompleted or StreamFailed cleaned up, this is a
		// no-op. A stream still tracked here ended without either — treat
		// the silent ending as a failure so it can never look like a hang.
		if _, tracked := model.Streams[message.Stream]; !tracked {
			return model, mvu.DoNothing()
		}
		return Update(model, StreamFailed{Stream: message.Stream, Err: "the response ended unexpectedly"})

	case HistLoaded:
		// Stale guard: apply only to the chat that asked, and never over a
		// live stream's history (the stream owns the view then).
		if message.Chat != model.CurrentChat.Name {
			return model, mvu.DoNothing()
		}
		if _, streaming := model.StreamFor(message.Chat); streaming {
			return model, mvu.DoNothing()
		}
		model.CurrentChat.History = message.History
		model.CurrentChat.Provider = message.Provider
		model.CurrentChat.Model = message.Model
		model.CurrentChat.Loaded = true
		return model, mvu.DoNothing()

	case ChatList:
		model.ChatList = message
		return model, mvu.DoNothing()

	case DeleteChat:
		index := slices.Index(model.ChatList, message.Name)
		if index < 0 {
			return model, mvu.DoNothing()
		}
		// Drop any in-flight completion for the deleted chat: later deltas
		// become untracked and are discarded (undo does not revive it).
		if id, ok := model.StreamFor(message.Name); ok {
			streams := cloneStreams(model.Streams)
			delete(streams, id)
			model.Streams = streams
		}
		model.ChatList = slices.Delete(slices.Clone(model.ChatList), index, index+1)
		wasCurrent := model.CurrentChat.Name == message.Name
		// The history file moves to the trash, where it stays undoable for
		// the whole session (Cmd/Ctrl-Z or the bar's Undo); the ExpireDelete
		// timer only hides the bar.
		model.DeleteGen++
		pending := PendingDelete{Name: message.Name, Index: index, WasCurrent: wasCurrent, Gen: model.DeleteGen}
		model.Pending = pending
		model.Trash = append(slices.Clone(model.Trash), pending)
		commands := []mvu.Command{
			TrashHist(model.ChatFile(message.Name), model.TrashFile(message.Name)).Trace("Trash History"),
			ExpireDelete(model.DeleteGen, UndoWindow).Trace("Undo Bar Timer"),
		}
		if wasCurrent {
			// The current chat was deleted: fall back to the first
			// remaining chat (adopting its live stream if it has one), or
			// to the empty state when none are left.
			if len(model.ChatList) > 0 {
				var load mvu.Command
				model, load = adoptStreamOrLoad(model, model.ChatList[0])
				commands = append(commands, load)
			} else {
				model.CurrentChat = Chat{}
			}
			commands = append(commands,
				SaveConfig(model.ConfigFile(), model.Config()).Trace("Save Config"),
			)
		}
		return model, mvu.DoConcurrent(commands...)

	case UndoDelete:
		// Pop the most recent delete off the session's trash stack — this
		// works whether or not the undo bar is still showing it.
		if len(model.Trash) == 0 {
			return model, mvu.DoNothing()
		}
		trash := slices.Clone(model.Trash)
		pending := trash[len(trash)-1]
		model.Trash = trash[:len(trash)-1]
		if model.Pending.Gen == pending.Gen {
			model.Pending = PendingDelete{} // hide the bar for this delete
		}
		index := min(pending.Index, len(model.ChatList))
		model.ChatList = slices.Insert(slices.Clone(model.ChatList), index, pending.Name)
		restore := RenameHist(model.TrashFile(pending.Name), model.ChatFile(pending.Name)).Trace("Restore History")
		if pending.WasCurrent {
			model = stashCurrentStream(model)
			// The restored chat cannot be streaming (its stream was
			// dropped at delete), so this is always a disk load.
			model.CurrentChat = Chat{Name: pending.Name}
			// Sequence: the history can only load AFTER the file is back.
			return model, mvu.DoSequence(
				restore,
				mvu.DoConcurrent(
					LoadHist(pending.Name, model.ChatFile(pending.Name)).Trace("Load Restored History"),
					SaveConfig(model.ConfigFile(), model.Config()).Trace("Save Config"),
				),
			)
		}
		return model, restore

	case ConfirmDelete:
		// The timer only hides the bar; the delete stays undoable. Ignore
		// stale generations (the bar was replaced or already dismissed).
		if model.Pending.Gen == message.Gen {
			model.Pending = PendingDelete{}
		}
		return model, mvu.DoNothing()

	case NewChat:
		name := FreshChatName(model.TakenNames())
		model.ChatList = append(slices.Clone(model.ChatList), name)
		model.CurrentChat = Chat{Name: name, Loaded: true}
		return model, mvu.DoConcurrent(
			CreateChat(model.ChatFile(name)).Trace("Create Chat"),
			SaveConfig(model.ConfigFile(), model.Config()).Trace("Save Config"),
		)

	case OpenRename:
		if !slices.Contains(model.ChatList, message.Name) {
			return model, mvu.DoNothing()
		}
		model.Rename = RenameState{Target: message.Name, Epoch: model.Rename.Epoch + 1}
		return model, mvu.DoNothing()

	case CloseRename:
		model.Rename.Target = ""
		return model, mvu.DoNothing()

	case RenameChat:
		target := model.Rename.Target
		if target == "" {
			return model, mvu.DoNothing()
		}
		to := strings.TrimSpace(message.To)
		if to == "" {
			// Empty submit keeps the old name and just closes the modal.
			model.Rename.Target = ""
			return model, mvu.DoNothing()
		}
		if strings.ContainsAny(to, `/\`) {
			// Names are filenames; path separators are invalid. The modal
			// stays open for another attempt.
			return model, mvu.DoNothing()
		}
		newName := to
		switch lower := strings.ToLower(newName); {
		case strings.HasSuffix(lower, ".jsonl"):
		case strings.HasSuffix(lower, ".json"):
			// A typed legacy extension follows the store to .jsonl.
			newName = newName[:len(newName)-len(".json")] + ".jsonl"
		default:
			newName += ".jsonl"
		}
		if newName == target {
			model.Rename.Target = ""
			return model, mvu.DoNothing()
		}
		index := slices.Index(model.ChatList, target)
		if index < 0 {
			// The target was deleted while the modal was open.
			model.Rename.Target = ""
			return model, mvu.DoNothing()
		}
		if slices.Contains(model.TakenNames(), newName) {
			// Name taken (listed or still undoable in the trash); the modal
			// stays open for another attempt.
			return model, mvu.DoNothing()
		}
		list := slices.Clone(model.ChatList)
		list[index] = newName
		model.ChatList = list
		model.Rename.Target = ""
		// An in-flight completion follows its chat to the new name (deltas
		// are routed by stream id, so nothing else needs to change).
		if id, ok := model.StreamFor(target); ok {
			streams := cloneStreams(model.Streams)
			s := streams[id]
			s.Chat = newName
			streams[id] = s
			model.Streams = streams
		}
		commands := []mvu.Command{RenameHist(model.ChatFile(target), model.ChatFile(newName)).Trace("Rename History")}
		if model.CurrentChat.Name == target {
			model.CurrentChat.Name = newName
			commands = append(commands,
				SaveConfig(model.ConfigFile(), model.Config()).Trace("Save Config"),
			)
		}
		return model, mvu.DoConcurrent(commands...)

	case ToggleSidebar:
		model.SidebarCollapsed = !model.SidebarCollapsed
		return model, SaveConfig(model.ConfigFile(), model.Config()).Trace("Save Config")

	case SetSidebarRatio:
		// Dragging below the rail threshold collapses; dragging wider both
		// restores and remembers the new width. The stored ratio survives a
		// collapse so the toggle restores the previous width.
		if message.Ratio > RailThresholdRatio {
			model.SidebarRatio = message.Ratio
			model.SidebarCollapsed = false
		} else {
			model.SidebarCollapsed = true
		}
		return model, mvu.DoNothing()

	case OpenSettings:
		selected := 0
		for i, p := range model.Providers {
			if p.Name == model.DefaultProvider {
				selected = i
			}
		}
		model.Settings = SettingsState{
			Open:            true,
			Epoch:           model.Settings.Epoch + 1,
			Draft:           slices.Clone(model.Providers),
			Selected:        selected,
			DefaultProvider: model.DefaultProvider,
			DefaultModel:    model.DefaultModel,
			Errors:          map[string]string{},
		}
		// Refresh every keyed provider's model list so the pickers are
		// current; cached lists keep the modal usable until results land.
		var commands []mvu.Command
		for _, p := range model.Providers {
			if p.APIKey != "" {
				commands = append(commands, FetchModels(p).Trace("Fetch Models"))
			}
		}
		return model, mvu.DoConcurrent(commands...)

	case CloseSettings:
		model.Settings.Open = false
		return model, mvu.DoNothing()

	case SelectProvider:
		if !model.Settings.Open || message.Index < 0 || message.Index >= len(model.Settings.Draft) || message.Index == model.Settings.Selected {
			return model, mvu.DoNothing()
		}
		model.Settings.Selected = message.Index
		model.Settings.Dropdown = false
		model.Settings.Epoch++
		return model, mvu.DoNothing()

	case ApplyTemplate:
		s := model.Settings
		p, ok := s.SelectedProvider()
		if !s.Open || !ok || message.Index < 0 || message.Index >= len(ProviderTemplates) {
			return model, mvu.DoNothing()
		}
		tpl := ProviderTemplates[message.Index]
		// Names key everything (default pair, fetch results), so a
		// template applied twice uniquifies against the OTHER entries.
		name := tpl.Name
		taken := func(n string) bool {
			for i, q := range s.Draft {
				if i != s.Selected && q.Name == n {
					return true
				}
			}
			return false
		}
		for i := 2; taken(name); i++ {
			name = fmt.Sprintf("%s %d", tpl.Name, i)
		}
		if s.DefaultProvider == p.Name {
			model.Settings.DefaultProvider = name
		}
		// The endpoint changes, so the cached model list and the key-check
		// result are both stale.
		model.Settings = clearKeyCheck(model.Settings, p.Name)
		p.Name, p.BaseURL, p.Models = name, tpl.BaseURL, nil
		draft := slices.Clone(s.Draft)
		draft[s.Selected] = p
		model.Settings.Draft = draft
		model.Settings.Dropdown = false
		model.Settings.Epoch++
		// A key already present is checked against the new endpoint right
		// away — a template click is a settled edit.
		command := mvu.DoNothing()
		if p.APIKey != "" {
			command = FetchModels(p).Trace("Fetch Models")
		}
		return model, command

	case OpenDefaultModelMenu:
		if model.Settings.Open {
			model.Settings.Dropdown = true
		}
		return model, mvu.DoNothing()

	case CloseDefaultModelMenu:
		model.Settings.Dropdown = false
		return model, mvu.DoNothing()

	case AddProvider:
		if !model.Settings.Open {
			return model, mvu.DoNothing()
		}
		model.Settings.Draft = append(slices.Clone(model.Settings.Draft), Provider{Name: FreshProviderName(model.Settings.Draft)})
		model.Settings.Selected = len(model.Settings.Draft) - 1
		model.Settings.Dropdown = false
		model.Settings.Epoch++
		return model, mvu.DoNothing()

	case RemoveProvider:
		s := model.Settings
		removed, ok := s.SelectedProvider()
		if !s.Open || !ok {
			return model, mvu.DoNothing()
		}
		model.Settings.Draft = slices.Delete(slices.Clone(s.Draft), s.Selected, s.Selected+1)
		model.Settings.Selected = min(s.Selected, len(model.Settings.Draft)-1)
		if s.DefaultProvider == removed.Name {
			model.Settings.DefaultProvider, model.Settings.DefaultModel = "", ""
		}
		model.Settings.Dropdown = false
		model.Settings.Epoch++
		return model, mvu.DoNothing()

	case EditProvider:
		s := model.Settings
		p, ok := s.SelectedProvider()
		if !s.Open || !ok {
			return model, mvu.DoNothing()
		}
		command := mvu.DoNothing()
		switch message.Field {
		case FieldName:
			// The draft default and the key-check result follow the provider
			// they name through the rename.
			if s.DefaultProvider == p.Name {
				model.Settings.DefaultProvider = message.Text
			}
			if err, checked := s.Errors[p.Name]; checked {
				errors := maps.Clone(s.Errors)
				delete(errors, p.Name)
				errors[message.Text] = err
				model.Settings.Errors = errors
			}
			p.Name = message.Text
		case FieldBaseURL:
			p.BaseURL = strings.TrimSpace(message.Text)
		case FieldAPIKey:
			p.APIKey = strings.TrimSpace(message.Text)
		}
		if message.Field == FieldBaseURL || message.Field == FieldAPIKey {
			// The endpoint or key changed, so the previous check no longer
			// applies: back to "checking", with a debounce timer that
			// re-checks by fetching /models once the typing settles.
			model.Settings = clearKeyCheck(model.Settings, p.Name)
			model.Settings.EditGen++
			command = SettleProviderEdit(model.Settings.EditGen, p.Name, EditSettleDelay)
		}
		draft := slices.Clone(s.Draft)
		draft[s.Selected] = p
		model.Settings.Draft = draft
		return model, command

	case ProviderEditSettled:
		s := model.Settings
		if !s.Open || message.Gen != s.EditGen {
			return model, mvu.DoNothing()
		}
		// The provider may have been renamed or removed since the timer was
		// armed; the check is simply dropped then (a later edit re-arms it).
		i := slices.IndexFunc(s.Draft, func(p Provider) bool { return p.Name == message.Provider })
		if i < 0 || s.Draft[i].APIKey == "" {
			return model, mvu.DoNothing()
		}
		return model, FetchModels(s.Draft[i]).Trace("Fetch Models")

	case ToggleWebSearch:
		s := model.Settings
		p, ok := s.SelectedProvider()
		if !s.Open || !ok {
			return model, mvu.DoNothing()
		}
		p.WebSearch = !p.WebSearch
		draft := slices.Clone(s.Draft)
		draft[s.Selected] = p
		model.Settings.Draft = draft
		return model, mvu.DoNothing()

	case SetDefaultModel:
		if !model.Settings.Open {
			return model, mvu.DoNothing()
		}
		model.Settings.DefaultProvider = message.Provider
		model.Settings.DefaultModel = message.Model
		model.Settings.Dropdown = false
		return model, mvu.DoNothing()

	case RefreshModels:
		p, ok := model.Settings.SelectedProvider()
		if !model.Settings.Open || !ok || p.APIKey == "" {
			return model, mvu.DoNothing()
		}
		// Back to "checking" while the manual re-fetch is in flight.
		model.Settings = clearKeyCheck(model.Settings, p.Name)
		return model, FetchModels(p).Trace("Fetch Models")

	case ModelsFetched:
		command := mvu.DoNothing()
		if message.Err == "" {
			// Cache into the LIVE catalogue (persisted) so pickers have
			// entries offline; the draft updates too while the modal is
			// open. Matching is by name; a renamed provider drops the
			// stale result.
			if i := slices.IndexFunc(model.Providers, func(p Provider) bool { return p.Name == message.Provider }); i >= 0 {
				providers := slices.Clone(model.Providers)
				providers[i].Models = message.Models
				model.Providers = providers
				command = SaveConfig(model.ConfigFile(), model.Config()).Trace("Save Config")
			}
		}
		if model.Settings.Open {
			if i := slices.IndexFunc(model.Settings.Draft, func(p Provider) bool { return p.Name == message.Provider }); i >= 0 {
				if message.Err == "" {
					draft := slices.Clone(model.Settings.Draft)
					draft[i].Models = message.Models
					model.Settings.Draft = draft
				}
				errors := maps.Clone(model.Settings.Errors)
				errors[message.Provider] = message.Err
				model.Settings.Errors = errors
			}
		}
		return model, command

	case SaveSettings:
		s := model.Settings
		if !s.Open {
			return model, mvu.DoNothing()
		}
		model.Providers = s.Draft
		model.DefaultProvider, model.DefaultModel = s.DefaultProvider, s.DefaultModel
		// A default that no longer names a draft provider falls back to
		// the first provider (or nothing).
		if _, ok := model.ProviderNamed(model.DefaultProvider); !ok {
			model.DefaultProvider, model.DefaultModel = "", ""
			if len(model.Providers) > 0 {
				model.DefaultProvider = model.Providers[0].Name
				if len(model.Providers[0].Models) > 0 {
					model.DefaultModel = model.Providers[0].Models[0]
				}
			}
		}
		model.Settings.Open = false
		commands := []mvu.Command{SaveConfig(model.ConfigFile(), model.Config()).Trace("Save Config")}
		// Refresh caches for the saved catalogue (new keys/urls included).
		for _, p := range model.Providers {
			if p.APIKey != "" {
				commands = append(commands, FetchModels(p).Trace("Fetch Models"))
			}
		}
		return model, mvu.DoConcurrent(commands...)

	case OpenModelMenu:
		model.ModelMenu = true
		return model, mvu.DoNothing()

	case CloseModelMenu:
		model.ModelMenu = false
		return model, mvu.DoNothing()

	case SetChatModel:
		model.ModelMenu = false
		// No chat, or one whose history is still loading from disk — the
		// meta event must not land in a file the user cannot yet see.
		if model.CurrentChat.Name == "" || !model.CurrentChat.Loaded {
			return model, mvu.DoNothing()
		}
		model.CurrentChat.Provider = message.Provider
		model.CurrentChat.Model = message.Model
		// An in-flight exchange for this chat adopts the override so its
		// stream state keeps mirroring the chat.
		if id, ok := model.StreamFor(model.CurrentChat.Name); ok {
			streams := cloneStreams(model.Streams)
			s := streams[id]
			s.Provider, s.Model = message.Provider, message.Model
			streams[id] = s
			model.Streams = streams
		}
		return model, AppendChatEvent(model.ChatFile(model.CurrentChat.Name), ChatEvent{
			Type: "meta", Provider: message.Provider, Model: message.Model,
		}).Trace("Append Model Override")

	case SelectChat:
		if model.CurrentChat.Name == message.Name {
			return model, mvu.DoNothing()
		}
		model = stashCurrentStream(model)
		model, load := adoptStreamOrLoad(model, message.Name)
		command := mvu.DoConcurrent(
			load,
			SaveConfig(model.ConfigFile(), model.Config()).Trace("Save Config"),
		)
		return model, command
	}
	return model, mvu.DoNothing()
}

// streamHist picks the history a stream's events apply to: the visible
// history while its chat is current, else the stream's own buffer.
func streamHist(model Model, s StreamState) []Message {
	if s.Chat == model.CurrentChat.Name {
		return model.CurrentChat.History
	}
	return s.History
}

// storeStreamHist writes the (possibly reallocated) history back where
// streamHist took it from.
func storeStreamHist(model Model, id int, s StreamState, hist []Message) Model {
	if s.Chat == model.CurrentChat.Name {
		model.CurrentChat.History = hist
		return model
	}
	streams := cloneStreams(model.Streams)
	s.History = hist
	streams[id] = s
	model.Streams = streams
	return model
}

// clearKeyCheck drops the provider's recorded /models outcome, returning
// its key status to "checking" until a fresh result lands.
func clearKeyCheck(s SettingsState, name string) SettingsState {
	if _, checked := s.Errors[name]; checked {
		errors := maps.Clone(s.Errors)
		delete(errors, name)
		s.Errors = errors
	}
	return s
}

// stashCurrentStream parks the visible history into the current chat's
// in-flight stream buffer (if any), so its deltas keep landing somewhere
// after the user switches away.
func stashCurrentStream(model Model) Model {
	if id, ok := model.StreamFor(model.CurrentChat.Name); ok {
		streams := cloneStreams(model.Streams)
		s := streams[id]
		s.History = model.CurrentChat.History
		streams[id] = s
		model.Streams = streams
	}
	return model
}

// adoptStreamOrLoad makes name the current chat: a chat with an in-flight
// completion adopts its live buffer (loading from disk would overwrite the
// stream) and the model override the stream carries, anything else starts
// a tagged disk load (which delivers the override with the history).
func adoptStreamOrLoad(model Model, name string) (Model, mvu.Command) {
	if id, ok := model.StreamFor(name); ok {
		streams := cloneStreams(model.Streams)
		s := streams[id]
		model.CurrentChat = Chat{Name: name, Provider: s.Provider, Model: s.Model, Loaded: true, History: s.History}
		s.History = nil
		streams[id] = s
		model.Streams = streams
		return model, mvu.DoNothing()
	}
	model.CurrentChat = Chat{Name: name}
	return model, LoadHist(name, model.ChatFile(name)).Trace("Load Selected History")
}
