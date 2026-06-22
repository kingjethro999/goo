package ai

import (
    "strings"
    "unicode"

    "github.com/yourusername/goo/memory"
)

type FollowUpSignal struct {
    Type       string
    Suggestion string
    AutoPrompt bool
}

type FollowUpAnalyser struct{}

func NewFollowUpAnalyser() *FollowUpAnalyser {
    return &FollowUpAnalyser{}
}

func (f *FollowUpAnalyser) Analyse(response string, history []memory.Message) []FollowUpSignal {
    var signals []FollowUpSignal

    trimmed := strings.TrimRightFunc(response, unicode.IsSpace)

    // Signal 1: Response ends with a question
    if strings.HasSuffix(trimmed, "?") {
        signals = append(signals, FollowUpSignal{
            Type:       "question",
            AutoPrompt: true,
        })
    }

    // Signal 2: Uncertainty language
    uncertaintyPhrases := []string{
        "i'm not sure", "it depends", "could you clarify",
        "not certain", "unclear", "you might want to check",
        "let me know if", "does that help",
    }
    lower := strings.ToLower(response)
    for _, phrase := range uncertaintyPhrases {
        if strings.Contains(lower, phrase) {
            signals = append(signals, FollowUpSignal{
                Type:       "uncertainty",
                Suggestion: "Want me to try a different angle, or can you give more context?",
                AutoPrompt: false,
            })
            break
        }
    }

    // Signal 3: Short response to a long / complex question
    if len(history) > 0 {
        lastUser := lastUserMessage(history)
        if wordCount(lastUser) > 20 && wordCount(response) < 80 {
            signals = append(signals, FollowUpSignal{
                Type:       "shallow",
                Suggestion: "Want me to go deeper on any part of this?",
                AutoPrompt: false,
            })
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
