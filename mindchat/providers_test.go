package main

import (
	"testing"
)

// providersModel is testModel with a two-provider catalogue, a default
// model, and a loaded current chat.
func providersModel() Model {
	m := testModel()
	m.CurrentChat.Loaded = true
	m.Providers = []Provider{
		{Name: "OpenAI", APIKey: "sk-1", Models: []string{"gpt-4o", "gpt-5.5"}},
		{Name: "xAI", BaseURL: "https://api.x.ai/v1", APIKey: "xk-1", Models: []string{"grok-4"}},
	}
	m.DefaultProvider, m.DefaultModel = "OpenAI", "gpt-5.5"
	return m
}

func TestConfigSeedsProviderFromEnvKey(t *testing.T) {
	next, _ := Update(Model{AuthToken: "tok"}, Config{})
	if len(next.Providers) != 1 || next.Providers[0].Name != "OpenAI" || next.Providers[0].APIKey != "tok" {
		t.Fatalf("Providers = %+v, want OpenAI seeded from the env key", next.Providers)
	}
	if next.DefaultProvider != "OpenAI" || next.DefaultModel != "gpt-5.5" {
		t.Fatalf("default = %q/%q, want OpenAI/gpt-5.5", next.DefaultProvider, next.DefaultModel)
	}
}

func TestConfigKeepsPersistedProviders(t *testing.T) {
	persisted := []Provider{{Name: "xAI", APIKey: "xk"}}
	next, _ := Update(Model{AuthToken: "tok"}, Config{Providers: persisted, DefaultProvider: "xAI", DefaultModel: "grok-4"})
	if len(next.Providers) != 1 || next.Providers[0].Name != "xAI" {
		t.Fatalf("Providers = %+v, want the persisted catalogue (no env seeding)", next.Providers)
	}
}

func TestOpenSettingsSeedsDraft(t *testing.T) {
	m := providersModel()
	m.DefaultProvider = "xAI"
	next, _ := Update(m, OpenSettings{})
	s := next.Settings
	if !s.Open || len(s.Draft) != 2 || s.Draft[0].Name != "OpenAI" {
		t.Fatalf("Settings = %+v, want open with the catalogue as draft", s)
	}
	if s.Selected != 1 {
		t.Fatalf("Selected = %d, want the default provider's index 1", s.Selected)
	}
	if s.DefaultProvider != "xAI" {
		t.Fatalf("draft default = %q, want xAI", s.DefaultProvider)
	}
}

func TestAddProviderSelectsFreshEntry(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	epoch := m.Settings.Epoch
	next, _ := Update(m, AddProvider{})
	s := next.Settings
	if len(s.Draft) != 3 || s.Draft[2].Name != "Provider" {
		t.Fatalf("Draft = %+v, want a fresh 'Provider' appended", s.Draft)
	}
	if s.Selected != 2 || s.Epoch != epoch+1 {
		t.Fatalf("Selected/Epoch = %d/%d, want the new entry selected under a new epoch", s.Selected, s.Epoch)
	}
}

func TestRemoveProviderClearsItsDefault(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	next, _ := Update(m, RemoveProvider{}) // selected = OpenAI, the default
	s := next.Settings
	if len(s.Draft) != 1 || s.Draft[0].Name != "xAI" {
		t.Fatalf("Draft = %+v, want only xAI left", s.Draft)
	}
	if s.DefaultProvider != "" || s.DefaultModel != "" {
		t.Fatalf("default = %q/%q, want cleared with its provider", s.DefaultProvider, s.DefaultModel)
	}
}

func TestEditProviderRenameFollowsDefault(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	next, _ := Update(m, EditProvider{Field: FieldName, Text: "OpenRouter"})
	s := next.Settings
	if s.Draft[0].Name != "OpenRouter" {
		t.Fatalf("Draft[0].Name = %q, want OpenRouter", s.Draft[0].Name)
	}
	if s.DefaultProvider != "OpenRouter" {
		t.Fatalf("draft default = %q, want to follow the rename", s.DefaultProvider)
	}
	if next.Providers[0].Name != "OpenAI" {
		t.Fatalf("live catalogue changed before Save: %+v", next.Providers)
	}
}

func TestSaveSettingsAppliesDraftAndDefault(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	m, _ = Update(m, SetDefaultModel{Provider: "xAI", Model: "grok-4"})
	next, _ := Update(m, SaveSettings{})
	if next.Settings.Open {
		t.Fatalf("settings still open after Save")
	}
	if next.DefaultProvider != "xAI" || next.DefaultModel != "grok-4" {
		t.Fatalf("default = %q/%q, want xAI/grok-4", next.DefaultProvider, next.DefaultModel)
	}
}

func TestSaveSettingsFallsBackWhenDefaultGone(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	m, _ = Update(m, RemoveProvider{}) // removes OpenAI, the default
	next, _ := Update(m, SaveSettings{})
	if next.DefaultProvider != "xAI" || next.DefaultModel != "grok-4" {
		t.Fatalf("default = %q/%q, want fallback to the first provider", next.DefaultProvider, next.DefaultModel)
	}
}

func TestKeyStatusClassification(t *testing.T) {
	s := SettingsState{Errors: map[string]string{"ok": "", "bad": "401"}}
	cases := []struct {
		p    Provider
		want KeyStatus
	}{
		{Provider{Name: "ok"}, KeyMissing}, // a missing key trumps any result
		{Provider{Name: "new", APIKey: "k"}, KeyChecking},
		{Provider{Name: "ok", APIKey: "k"}, KeyOK},
		{Provider{Name: "bad", APIKey: "k"}, KeyBad},
	}
	for _, c := range cases {
		if got := s.KeyStatus(c.p); got != c.want {
			t.Errorf("KeyStatus(%q) = %v, want %v", c.p.Name, got, c.want)
		}
	}
}

func TestEditKeyResetsCheckAndArmsDebounce(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	m, _ = Update(m, ModelsFetched{Provider: "OpenAI", Models: []string{"gpt-5.5"}})
	if got := m.Settings.KeyStatus(m.Settings.Draft[0]); got != KeyOK {
		t.Fatalf("status = %v, want KeyOK after a successful fetch", got)
	}
	gen := m.Settings.EditGen
	next, _ := Update(m, EditProvider{Field: FieldAPIKey, Text: "sk-new"})
	if next.Settings.EditGen != gen+1 {
		t.Fatalf("EditGen = %d, want a bump arming the settle timer", next.Settings.EditGen)
	}
	if got := next.Settings.KeyStatus(next.Settings.Draft[0]); got != KeyChecking {
		t.Fatalf("status = %v, want KeyChecking after the key changed", got)
	}
}

func TestRenameMigratesKeyCheck(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	m, _ = Update(m, ModelsFetched{Provider: "OpenAI", Err: "401 unauthorized"})
	next, _ := Update(m, EditProvider{Field: FieldName, Text: "OpenAI EU"})
	if next.Settings.Errors["OpenAI EU"] != "401 unauthorized" {
		t.Fatalf("Errors = %+v, want the check result to follow the rename", next.Settings.Errors)
	}
	if _, stale := next.Settings.Errors["OpenAI"]; stale {
		t.Fatalf("Errors = %+v, want the old name's entry gone", next.Settings.Errors)
	}
}

func TestApplyTemplateResetsKeyCheck(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	m, _ = Update(m, ModelsFetched{Provider: "OpenAI", Err: "401 unauthorized"})
	next, _ := Update(m, ApplyTemplate{Index: 3}) // Groq; the key stays as typed
	s := next.Settings
	if _, present := s.Errors["OpenAI"]; present {
		t.Fatalf("Errors = %+v, want the old endpoint's check dropped", s.Errors)
	}
	if got := s.KeyStatus(s.Draft[0]); got != KeyChecking {
		t.Fatalf("status = %v, want KeyChecking while the new endpoint is fetched", got)
	}
}

func TestModelsFetchedCachesLiveAndDraft(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	next, _ := Update(m, ModelsFetched{Provider: "xAI", Models: []string{"grok-4", "grok-5"}})
	if len(next.Providers[1].Models) != 2 {
		t.Fatalf("live cache = %+v, want the fetched ids", next.Providers[1].Models)
	}
	if len(next.Settings.Draft[1].Models) != 2 {
		t.Fatalf("draft cache = %+v, want the fetched ids", next.Settings.Draft[1].Models)
	}
	if next.Settings.Errors["xAI"] != "" {
		t.Fatalf("error = %q, want cleared on success", next.Settings.Errors["xAI"])
	}
}

func TestModelsFetchedRecordsError(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	next, _ := Update(m, ModelsFetched{Provider: "xAI", Err: "401 unauthorized"})
	if next.Settings.Errors["xAI"] != "401 unauthorized" {
		t.Fatalf("Errors = %+v, want the fetch error recorded", next.Settings.Errors)
	}
	if len(next.Providers[1].Models) != 1 {
		t.Fatalf("live cache = %+v, want untouched on error", next.Providers[1].Models)
	}
}

func TestSetChatModelSetsOverrideAndStream(t *testing.T) {
	m, _ := Update(providersModel(), Prompt{Content: "hello"}) // registers stream 1
	next, _ := Update(m, SetChatModel{Provider: "xAI", Model: "grok-4"})
	if next.CurrentChat.Provider != "xAI" || next.CurrentChat.Model != "grok-4" {
		t.Fatalf("override = %q/%q, want xAI/grok-4", next.CurrentChat.Provider, next.CurrentChat.Model)
	}
	s := next.Streams[next.NextStream]
	if s.Provider != "xAI" || s.Model != "grok-4" {
		t.Fatalf("stream override = %q/%q, want to follow the chat", s.Provider, s.Model)
	}
}

func TestSetChatModelNoOpWhileLoading(t *testing.T) {
	m := providersModel()
	m.CurrentChat.Loaded = false
	next, _ := Update(m, SetChatModel{Provider: "xAI", Model: "grok-4"})
	if next.CurrentChat.Provider != "" {
		t.Fatalf("override applied while the chat was still loading")
	}
}

func TestEffectiveModelResolution(t *testing.T) {
	m := providersModel()
	if p, id, ok := m.EffectiveModel(); !ok || p.Name != "OpenAI" || id != "gpt-5.5" {
		t.Fatalf("default resolution = %v/%q/%v, want OpenAI/gpt-5.5", p.Name, id, ok)
	}
	m.CurrentChat.Provider, m.CurrentChat.Model = "xAI", "grok-4"
	if p, id, _ := m.EffectiveModel(); p.Name != "xAI" || id != "grok-4" {
		t.Fatalf("override resolution = %v/%q, want xAI/grok-4", p.Name, id)
	}
	m.CurrentChat.Provider = "Gone"
	if p, _, _ := m.EffectiveModel(); p.Name != "OpenAI" {
		t.Fatalf("stale override resolution = %v, want fallback to the default", p.Name)
	}
	if _, _, ok := (Model{}).EffectiveModel(); ok {
		t.Fatalf("no providers must resolve to ok=false")
	}
}

func TestHistLoadedAppliesOverride(t *testing.T) {
	m := providersModel()
	m.CurrentChat = Chat{Name: "alpha.jsonl"}
	next, _ := Update(m, HistLoaded{Chat: "alpha.jsonl", Provider: "xAI", Model: "grok-4"})
	c := next.CurrentChat
	if c.Provider != "xAI" || c.Model != "grok-4" || !c.Loaded {
		t.Fatalf("chat = %+v, want the persisted override applied and Loaded set", c)
	}
}

func TestApplyTemplatePrefillsSelectedProvider(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	m, _ = Update(m, AddProvider{})
	epoch := m.Settings.Epoch
	next, _ := Update(m, ApplyTemplate{Index: 2}) // OpenRouter
	s := next.Settings
	p := s.Draft[s.Selected]
	if p.Name != "OpenRouter" || p.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("provider = %+v, want the OpenRouter template applied", p)
	}
	if s.Epoch != epoch+1 {
		t.Fatalf("Epoch = %d, want a bump so the fields reseed", s.Epoch)
	}
}

func TestApplyTemplateUniquifiesTakenName(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	m, _ = Update(m, AddProvider{})
	next, _ := Update(m, ApplyTemplate{Index: 1}) // xAI, already in the draft
	s := next.Settings
	if got := s.Draft[s.Selected].Name; got != "xAI 2" {
		t.Fatalf("Name = %q, want uniquified against the existing xAI", got)
	}
}

func TestDefaultModelDropdownOpensAndCloses(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	m, _ = Update(m, OpenDefaultModelMenu{})
	if !m.Settings.Dropdown {
		t.Fatalf("dropdown did not open")
	}
	m, _ = Update(m, SetDefaultModel{Provider: "OpenAI", Model: "gpt-4o"})
	if m.Settings.Dropdown {
		t.Fatalf("picking a model must close the dropdown")
	}
	if m.Settings.DefaultProvider != "OpenAI" || m.Settings.DefaultModel != "gpt-4o" {
		t.Fatalf("default = %q/%q, want OpenAI/gpt-4o", m.Settings.DefaultProvider, m.Settings.DefaultModel)
	}
	m, _ = Update(m, OpenDefaultModelMenu{})
	m, _ = Update(m, SelectProvider{Index: 1})
	if m.Settings.Dropdown {
		t.Fatalf("switching provider must close the dropdown")
	}
}

func TestIsChatModelFiltersNonChatIds(t *testing.T) {
	chat := []string{
		// xAI (live /models ids, 2026-07)
		"grok-4.20-0309-reasoning", "grok-4.20-multi-agent-0309", "grok-4.3", "grok-4.5", "grok-build-0.1",
		// OpenAI
		"gpt-5.5", "gpt-5-mini", "gpt-5.2-codex", "o3", "chatgpt-4o-latest",
	}
	for _, id := range chat {
		if !IsChatModel(id) {
			t.Errorf("IsChatModel(%q) = false, want chat model kept", id)
		}
	}
	nonChat := []string{
		// xAI
		"grok-imagine-image", "grok-imagine-image-quality", "grok-imagine-video", "grok-imagine-video-1.5",
		// OpenAI
		"gpt-4o-transcribe", "gpt-4o-transcribe-diarize", "gpt-4o-mini-tts", "whisper-1",
		"text-embedding-3-large", "dall-e-3", "omni-moderation-latest",
		"gpt-4o-realtime-preview", "gpt-audio", "babbage-002", "davinci-002",
	}
	for _, id := range nonChat {
		if IsChatModel(id) {
			t.Errorf("IsChatModel(%q) = true, want filtered out", id)
		}
	}
}

func TestParseChatFileAllFormats(t *testing.T) {
	legacy := []byte(`[{"role":"user","content":"hi"}]`)
	cf, err := ParseChatFile(legacy)
	if err != nil || len(cf.History) != 1 || cf.Provider != "" {
		t.Fatalf("legacy parse = %+v, %v", cf, err)
	}
	wrapped := []byte(`{"Provider":"xAI","Model":"grok-4","History":[{"role":"user","content":"hi"}]}`)
	cf, err = ParseChatFile(wrapped)
	if err != nil || cf.Provider != "xAI" || cf.Model != "grok-4" || len(cf.History) != 1 {
		t.Fatalf("wrapped parse = %+v, %v", cf, err)
	}
	jsonl := []byte(`{"time":"2026-07-19T10:00:00Z","type":"meta","provider":"xAI","model":"grok-4.5"}
{"time":"2026-07-19T10:00:01Z","type":"user","text":"search this"}
{"time":"2026-07-19T10:00:02Z","type":"tool","text":"future event kind"}
{"time":"2026-07-19T10:00:03Z","type":"assistant","text":"found it","citations":[{"url":"https://x.ai","title":"xAI"}]}
{"time":"2026-07-19T10:00:04Z","type":"error","error":"HTTP 410: Gone"}`)
	cf, err = ParseChatFile(jsonl)
	if err != nil || cf.Provider != "xAI" || cf.Model != "grok-4.5" {
		t.Fatalf("jsonl parse = %+v, %v (want the meta override applied)", cf, err)
	}
	// The unknown "tool" type is skipped; user, assistant and error replay.
	if len(cf.History) != 3 {
		t.Fatalf("jsonl history = %+v, want user+assistant+error rows", cf.History)
	}
	if cf.History[1].Role != RoleAssistant || len(cf.History[1].Citations) != 1 || cf.History[1].Citations[0].URL != "https://x.ai" {
		t.Fatalf("assistant row = %+v, want the citation attached", cf.History[1])
	}
	if cf.History[2].Role != RoleError || cf.History[2].Content != "HTTP 410: Gone" {
		t.Fatalf("error row = %+v, want the persisted error notice", cf.History[2])
	}
	empty, err := ParseChatFile(nil)
	if err != nil || len(empty.History) != 0 {
		t.Fatalf("empty parse = %+v, %v (a fresh chat file is empty)", empty, err)
	}
	single := []byte(`{"time":"2026-07-19T10:00:00Z","type":"user","text":"hi"}`)
	cf, err = ParseChatFile(single)
	if err != nil || len(cf.History) != 1 || cf.History[0].Role != RoleUser {
		t.Fatalf("single-line jsonl parse = %+v, %v (must not read as a legacy object)", cf, err)
	}
}

func TestToggleWebSearchFlipsDraftProvider(t *testing.T) {
	m, _ := Update(providersModel(), OpenSettings{})
	next, _ := Update(m, ToggleWebSearch{})
	if !next.Settings.Draft[0].WebSearch {
		t.Fatalf("WebSearch = false, want enabled after toggle")
	}
	if next.Providers[0].WebSearch {
		t.Fatalf("live catalogue changed before Save")
	}
	again, _ := Update(next, ToggleWebSearch{})
	if again.Settings.Draft[0].WebSearch {
		t.Fatalf("second toggle must disable")
	}
}
