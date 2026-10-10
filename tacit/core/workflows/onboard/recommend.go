package onboard

import (
	"context"
	"fmt"
	"os"
	"strings"

	modeldownloader "github.com/sangmin7648/tacit/core/internal/components/model-downloader"
	noteclassifier "github.com/sangmin7648/tacit/core/internal/components/note-classifier"
	settingmanager "github.com/sangmin7648/tacit/core/internal/components/setting-manager"
	skillinstaller "github.com/sangmin7648/tacit/core/internal/components/skill-installer"
)

// LocalModelRAMGB is the memory a Mac needs to run the recommended Ollama
// model beside the speech model and everything else: the model is 6.6 GB on
// disk and loads somewhat larger, Whisper takes about 2 GB, and macOS needs
// the rest. Below it the model still runs, slowly, so it is a warning and not
// a reason to recommend something that leaves the Mac.
const LocalModelRAMGB = 16

type (
	// OllamaStatus is what was found of Ollama on this Mac.
	OllamaStatus = noteclassifier.OllamaStatus
	// Agent is an AI agent skills can be installed for.
	Agent = skillinstaller.Agent
)

// Reason keys: Recommendation.Reasons says why each answer is recommended.
const (
	ReasonProvider = "llm_provider"
	ReasonModel    = "llm_model"
	ReasonAgent    = "skill_agent"
	ReasonLanguage = "language"
	ReasonWhisper  = "whisper_model"
)

// WhisperOption is a speech model with what the machine knows about it.
type WhisperOption struct {
	modeldownloader.WhisperModel
	Installed   bool `json:"installed"`
	Recommended bool `json:"recommended"`
}

// AgentOption is an agent with whether it was found on this Mac.
type AgentOption struct {
	Agent
	Installed   bool `json:"installed"`
	Recommended bool `json:"recommended"`
}

// Recommendation is what the window shows for every question: the answer
// recommended for this Mac, why, and the facts behind it. It never replaces
// the user's own choice; front ends show it beside the choice.
type Recommendation struct {
	Choices Choices           `json:"choices"`
	Reasons map[string]string `json:"reasons"`

	// MemoryWarning is set when this Mac has less memory than the local model
	// is comfortable with.
	MemoryWarning   string          `json:"memory_warning,omitempty"`
	ClaudeAvailable bool            `json:"claude_available"`
	Ollama          OllamaStatus    `json:"ollama"`
	Agents          []AgentOption   `json:"agents"`
	WhisperModels   []WhisperOption `json:"whisper_models"`
	MemoryGB        int             `json:"memory_gb"`
}

// Recommend inspects this Mac and recommends an answer to each question.
func Recommend(ctx context.Context) *Recommendation {
	ram := modeldownloader.SystemMemory()
	r := &Recommendation{
		Choices:         Defaults(),
		Reasons:         map[string]string{},
		MemoryGB:        int((ram + 1<<29) >> 30),
		ClaudeAvailable: noteclassifier.ClaudeAvailable(),
		Ollama:          noteclassifier.DetectOllama(ctx),
	}
	r.recommendClassifier(ram)
	r.recommendAgent()
	r.recommendLanguage(settingmanager.PreferredLanguages())
	r.recommendWhisper(ram)
	return r
}

// recommendClassifier always recommends the local model, because keeping what
// the user says on their Mac is what tacit is for: Ollama not being installed
// yet is something to fix, not a reason to send transcripts elsewhere. A Mac
// short of memory is told so, and left to decide.
func (r *Recommendation) recommendClassifier(ramBytes int64) {
	c := &r.Choices
	c.LLMProvider, c.LLMModel = "ollama", DefaultOllamaModel
	switch {
	case r.Ollama.Running:
		r.Reasons[ReasonProvider] = "Ollama is running here, so summaries stay on this Mac."
	case r.Ollama.Installed:
		r.Reasons[ReasonProvider] = "Ollama is installed but not running. Open it, then look again; summaries stay on this Mac."
	default:
		r.Reasons[ReasonProvider] = "Ollama was not found. It keeps summaries on this Mac: install it from ollama.com, then look again."
	}
	if r.Ollama.Running && HasOllamaModel(r.Ollama.Models, DefaultOllamaModel) {
		r.Reasons[ReasonModel] = DefaultOllamaModel + " is already installed. It gave the best titles and categories in our testing."
	} else {
		r.Reasons[ReasonModel] = DefaultOllamaModel + " gave the best titles and categories in our testing."
		if r.Ollama.Running {
			r.Reasons[ReasonModel] += " It is not installed yet; download it below."
		}
	}
	if ramBytes > 0 && ramBytes < LocalModelRAMGB<<30 {
		r.MemoryWarning = fmt.Sprintf("This Mac has %d GB of RAM, and %s works best with about %d GB beside the speech model, so it may run slowly. A smaller Ollama model is lighter.", r.MemoryGB, DefaultOllamaModel, LocalModelRAMGB)
		if r.ClaudeAvailable {
			r.MemoryWarning += " Claude is lighter still, but sends transcript text to Anthropic."
		}
	}
}

func (r *Recommendation) recommendAgent() {
	pick := -1
	for i, a := range skillinstaller.Agents {
		installed := a.Installed()
		if installed && pick < 0 {
			pick = i
		}
		r.Agents = append(r.Agents, AgentOption{Agent: a, Installed: installed})
	}
	if pick < 0 {
		pick = 0
		r.Reasons[ReasonAgent] = fmt.Sprintf("No supported agent was found on this Mac. %s is the default; the skills are ready for when you install it.", skillinstaller.Agents[0].Label)
	} else {
		r.Reasons[ReasonAgent] = fmt.Sprintf("%s is installed, so your notes can be searched from it.", skillinstaller.Agents[pick].Label)
	}
	r.Agents[pick].Recommended = true
	r.Choices.SkillAgent = skillinstaller.Agents[pick].Name
}

// recommendLanguage fixes the language to the one the user's Mac is set to,
// because fixing it cuts wrong-language transcriptions. When the Mac lists
// several offered languages the user switches between them, so detection stays
// on.
func (r *Recommendation) recommendLanguage(preferred []string) {
	var found []string
	for _, tag := range preferred {
		primary := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
		for _, l := range Languages[1:] {
			if l.Code == primary && !contains(found, primary) {
				found = append(found, primary)
			}
		}
	}
	switch len(found) {
	case 1:
		r.Choices.Language = found[0]
		r.Reasons[ReasonLanguage] = "Your Mac is set to " + languageLabel(found[0]) + ". Fixing the language cuts wrong-language transcriptions."
	case 0:
		r.Choices.Language = "auto"
		r.Reasons[ReasonLanguage] = "Your Mac's languages are not ones we can fix, so Tacit detects the language as you speak."
	default:
		r.Choices.Language = "auto"
		r.Reasons[ReasonLanguage] = "Your Mac lists " + strings.Join(found, " and ") + ", so Tacit detects the language as you speak. Pick one if you only speak that."
	}
}

func (r *Recommendation) recommendWhisper(ramBytes int64) {
	name := modeldownloader.RecommendWhisperModel(ramBytes)
	r.Choices.WhisperModel = name

	var rec modeldownloader.WhisperModel
	for _, m := range modeldownloader.WhisperModels {
		_, err := os.Stat(settingmanager.ModelPath(m.Name))
		r.WhisperModels = append(r.WhisperModels, WhisperOption{WhisperModel: m, Installed: err == nil, Recommended: m.Name == name})
		if m.Name == name {
			rec = m
		}
	}
	use := fmt.Sprintf("%s uses about %s of RAM while transcribing", name, memoryText(rec.RAMMB))
	switch {
	case ramBytes == 0:
		r.Reasons[ReasonWhisper] = "We could not read this Mac's memory. " + use + "."
	case name == "large-v3-turbo":
		r.Reasons[ReasonWhisper] = fmt.Sprintf("This Mac has %d GB of RAM. %s and is the most accurate choice that fits.", r.MemoryGB, use)
	default:
		r.Reasons[ReasonWhisper] = fmt.Sprintf("This Mac has %d GB of RAM. %s, leaving room for your other apps.", r.MemoryGB, use)
	}
}

func memoryText(mb int) string {
	if mb >= 1000 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1000)
	}
	return fmt.Sprintf("%d MB", mb)
}

func languageLabel(code string) string {
	for _, l := range Languages {
		if l.Code == code {
			return l.Label
		}
	}
	return code
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// PullOllamaModel downloads model into the local Ollama, for the window's
// "download" button next to the recommended model.
func PullOllamaModel(ctx context.Context, model string, progress Progress) error {
	return noteclassifier.PullOllamaModel(ctx, model, progress)
}

// HasOllamaModel reports whether the models Ollama lists include model.
func HasOllamaModel(models []string, model string) bool {
	return noteclassifier.HasOllamaModel(models, model)
}
