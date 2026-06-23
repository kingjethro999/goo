package ai

import (
	"strings"
	"unicode"

	"github.com/kingjethro999/goo/memory"
)

const (
	SignalQuestion  = "question"
	SignalUncertain = "uncertainty"
	SignalShallow   = "shallow"
	SignalReference = "reference"
	SignalConflict  = "conflict"
)

// FollowUpSignal represents a detected follow-up opportunity.
type FollowUpSignal struct {
	Type       string
	Suggestion string
	AutoPrompt bool // if true, display immediately; if false, show dimly
}

// FollowUpAnalyser scans AI responses for signals that the conversation needs continuation.
type FollowUpAnalyser struct{}

// NewFollowUpAnalyser creates a new analyser.
func NewFollowUpAnalyser() *FollowUpAnalyser { return &FollowUpAnalyser{} }

// Analyse returns any detected follow-up signals.
func (f *FollowUpAnalyser) Analyse(response string, history []memory.Message) []FollowUpSignal {
	var signals []FollowUpSignal
	trimmed := strings.TrimRightFunc(response, unicode.IsSpace)

	// Signal 1: Response ends with a question
	if strings.HasSuffix(trimmed, "?") {
		signals = append(signals, FollowUpSignal{
			Type:       SignalQuestion,
			AutoPrompt: true,
		})
	}

	// Signal 2: Uncertainty language
	uncertaintyPhrases := []string{
		"i'm not sure", "it depends", "could you clarify",
		"not certain", "unclear", "you might want to check",
		"let me know if", "does that help", "i'm unsure",
	}
	lower := strings.ToLower(response)
	for _, phrase := range uncertaintyPhrases {
		if strings.Contains(lower, phrase) {
			signals = append(signals, FollowUpSignal{
				Type:       SignalUncertain,
				Suggestion: "Want me to try a different angle, or can you give more context?",
				AutoPrompt: false,
			})
			break
		}
	}

	// Signal 3: Short response to a long/complex question
	if len(history) > 0 {
		lastUser := lastUserMessage(history)
		if wordCount(lastUser) > 20 && wordCount(response) < 80 {
			signals = append(signals, FollowUpSignal{
				Type:       SignalShallow,
				Suggestion: "Want me to go deeper on any part of this?",
				AutoPrompt: false,
			})
		}
	}

	// Signal 4: AI mentioned creating a task or action item
	actionPhrases := []string{
		"you could create", "you might want to", "consider adding",
		"you should add", "don't forget to", "remember to",
		"it would help to", "i'd suggest",
	}
	for _, phrase := range actionPhrases {
		if strings.Contains(lower, phrase) {
			signals = append(signals, FollowUpSignal{
				Type:       SignalReference,
				Suggestion: "Want me to add that as a task?",
				AutoPrompt: false,
			})
			break
		}
	}

	return signals
}

func lastUserMessage(history []memory.Message) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "user" {
			return history[i].Content
		}
	}
	return ""
}

func wordCount(s string) int {
	return len(strings.Fields(s))
}
