package core

import (
    "bufio"
    "fmt"
    "os"
    "strings"

    "github.com/yourusername/goo/memory"
    "github.com/yourusername/goo/tools/ai"
    "github.com/yourusername/goo/core/renderer"
)

// RunChatSession starts and manages an interactive chat session.
func RunChatSession(session *memory.Session, store *memory.Store) error {
    r := renderer.New()
    groqClient, err := ai.NewGroqClient()
    if err != nil {
        return err
    }
    followup := ai.NewFollowUpAnalyser()
    ctx := buildContext(session, store)

    r.PrintSessionHeader(session)
    r.PrintHint("Type /exit to quit. Use /help for commands.")

    scanner := bufio.NewScanner(os.Stdin)
    for {
        r.PrintPrompt()
        if !scanner.Scan() {
            break
        }

        input := strings.TrimSpace(scanner.Text())
        if input == "" {
            continue
        }

        // Handle slash commands
        if strings.HasPrefix(input, "/") {
            if input == "/exit" || input == "/quit" {
                r.PrintInfo("Session saved. Goodbye!")
                return nil
            }
            if err := handleSlashCommand(input, session, store, r); err != nil {
                r.PrintError(err)
            }
            continue
        }

        // Save user message
        userMsg := memory.Message{
            Role:      "user",
            Content:   input,
            SessionID: session.ID,
        }
        if err := store.SaveMessage(userMsg); err != nil {
            return err
        }

        // Build context window
        messages := ctx.Build(input)

        // Stream response
        r.PrintAILabel()
        var fullResponse strings.Builder
        if err := groqClient.StreamChat(
            cmd.Context(),
            messages,
            r.StreamWriter(&fullResponse),
        ); err != nil {
            r.PrintError(fmt.Errorf("AI error: %w", err))
            continue
        }
        fmt.Println()

        // Save assistant message
        assistantMsg := memory.Message{
            Role:      "assistant",
            Content:   fullResponse.String(),
            SessionID: session.ID,
        }
        if err := store.SaveMessage(assistantMsg); err != nil {
            return err
        }

        // Analyse for follow-ups
        signals := followup.Analyse(fullResponse.String(), ctx.RecentMessages())
        for _, sig := range signals {
            if sig.AutoPrompt {
                r.PrintFollowUp(sig.Suggestion)
            }
        }
    }
    return nil
}
