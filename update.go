package main

import (
	"slices"
	"strings"

	"github.com/vibrantgio/mvu"

	openai "github.com/sashabaranov/go-openai"
)

// Update is the MVU update function: it reduces the model and returns the
// command whose messages the runner in main.go feeds back into the loop.
func Update(model Model, message mvu.Message) (Model, mvu.Command) {
	switch message := message.(type) {

	case Config:
		model.CurrentChat.Name = message.LastChat
		commands := []mvu.Command{LoadChatList(model.ChatDir()).Trace("Load Chat List")}
		// LastChat is empty when the last chat was deleted; there is no
		// history to load then.
		if message.LastChat != "" {
			commands = append(commands, LoadHist(message.LastChat, model.ChatFile(message.LastChat)).Trace("Load History"))
		}
		return model, mvu.DoConcurrent(commands...)

	case Prompt:
		command := mvu.DoNothing()
		if message.Content != "" {
			// Prompting with no chat selected (every chat was deleted)
			// starts a fresh one, so the completion has a file to persist to.
			if model.CurrentChat.Name == "" {
				model.CurrentChat.Name = FreshChatName(model.TakenNames())
				model.ChatList = append(slices.Clone(model.ChatList), model.CurrentChat.Name)
			}
			model.CurrentChat.History = append(model.CurrentChat.History, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: message.Content,
			})
			// Register the stream so its deltas stay routable to THIS chat
			// whatever is current when they arrive.
			model.NextStream++
			streams := cloneStreams(model.Streams)
			streams[model.NextStream] = StreamState{Chat: model.CurrentChat.Name}
			model.Streams = streams
			command = mvu.DoSequence(
				RequestChatCompletion(model.NextStream, model.AuthToken, model.CurrentChat.History).Trace("Request Chat Completion"),
				SaveConfig(model.ConfigFile(), Config{LastChat: model.CurrentChat.Name}).Trace("Save Config"),
			)
		}
		return model, command

	case CompletionDelta:
		s, tracked := model.Streams[message.Stream]
		if !tracked || len(message.Response.Choices) == 0 {
			// Untracked stream: its chat was deleted mid-stream. Drop the
			// delta; the connection drains harmlessly.
			return model, mvu.DoNothing()
		}
		choice := message.Response.Choices[0]
		current := s.Chat == model.CurrentChat.Name

		// The delta applies to the visible history only while its chat is
		// current; otherwise it accumulates in the stream's buffer.
		hist := s.History
		if current {
			hist = model.CurrentChat.History
		}
		if choice.Delta.Role != "" {
			hist = append(hist, openai.ChatCompletionMessage{
				Role:    choice.Delta.Role,
				Content: choice.Delta.Content,
			})
		} else if len(hist) > 0 {
			hist[len(hist)-1].Content += choice.Delta.Content
		}
		if current {
			model.CurrentChat.History = hist
		} else {
			streams := cloneStreams(model.Streams)
			s.History = hist
			streams[message.Stream] = s
			model.Streams = streams
		}

		command := mvu.DoNothing()
		if choice.FinishReason == openai.FinishReasonStop {
			// Save to the stream's OWN chat file — never to whatever chat
			// happens to be current.
			command = SaveHist(model.ChatFile(s.Chat), hist).Trace("Save History")
			streams := cloneStreams(model.Streams)
			delete(streams, message.Stream)
			model.Streams = streams
		}
		return model, command

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
			model.CurrentChat.History = nil
			if len(model.ChatList) > 0 {
				var load mvu.Command
				model, load = adoptStreamOrLoad(model, model.ChatList[0])
				commands = append(commands, load)
			} else {
				model.CurrentChat.Name = ""
			}
			commands = append(commands,
				SaveConfig(model.ConfigFile(), Config{LastChat: model.CurrentChat.Name}).Trace("Save Config"),
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
			model.CurrentChat.Name = pending.Name
			model.CurrentChat.History = nil
			// Sequence: the history can only load AFTER the file is back.
			return model, mvu.DoSequence(
				restore,
				mvu.DoConcurrent(
					LoadHist(pending.Name, model.ChatFile(pending.Name)).Trace("Load Restored History"),
					SaveConfig(model.ConfigFile(), Config{LastChat: pending.Name}).Trace("Save Config"),
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
		model.CurrentChat = Chat{Name: name}
		return model, mvu.DoConcurrent(
			SaveHist(model.ChatFile(name), []openai.ChatCompletionMessage{}).Trace("Create Chat"),
			SaveConfig(model.ConfigFile(), Config{LastChat: name}).Trace("Save Config"),
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
		if !strings.HasSuffix(strings.ToLower(newName), ".json") {
			newName += ".json"
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
				SaveConfig(model.ConfigFile(), Config{LastChat: newName}).Trace("Save Config"),
			)
		}
		return model, mvu.DoConcurrent(commands...)

	case OpenSettings:
		// Settings surface (OPENAI_API_KEY configuration) not built yet.
		return model, mvu.DoNothing()

	case SelectChat:
		if model.CurrentChat.Name == message.Name {
			return model, mvu.DoNothing()
		}
		model = stashCurrentStream(model)
		model, load := adoptStreamOrLoad(model, message.Name)
		command := mvu.DoConcurrent(
			load,
			SaveConfig(model.ConfigFile(), Config{LastChat: message.Name}).Trace("Save Config"),
		)
		return model, command
	}
	return model, mvu.DoNothing()
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
// stream), anything else starts a tagged disk load.
func adoptStreamOrLoad(model Model, name string) (Model, mvu.Command) {
	model.CurrentChat.Name = name
	if id, ok := model.StreamFor(name); ok {
		streams := cloneStreams(model.Streams)
		s := streams[id]
		model.CurrentChat.History = s.History
		s.History = nil
		streams[id] = s
		model.Streams = streams
		return model, mvu.DoNothing()
	}
	model.CurrentChat.History = nil
	return model, LoadHist(name, model.ChatFile(name)).Trace("Load Selected History")
}
