package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func main() {
	// 1. Generate full docs tree in /docs
	fmt.Println("Generating full Markdown docs in ./docs...")
	cmd := exec.Command("go", "run", "./cmd/gorinth", "docs")
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to run docs command: %v", err)
	}

	// 2. Read the main README.md
	readmePath := "README.md"
	content, err := os.ReadFile(readmePath)
	if err != nil {
		log.Fatalf("failed to read README: %v", err)
	}

	// 3. Capture the current help output for reference
	fmt.Println("Updating README.md command reference...")
	helpOutput, err := exec.Command("go", "run", "./cmd/gorinth", "--help").Output()
	if err != nil {
		log.Fatalf("failed to get help output: %v", err)
	}

	// 4. Wrap help output in a code block
	formattedHelp := fmt.Sprintf("```text\n%s\n```", strings.TrimSpace(string(helpOutput)))

	// 5. Replace the content between START_COMMAND_REFERENCE and END_COMMAND_REFERENCE
	re := regexp.MustCompile(`(?s)<!-- START_COMMAND_REFERENCE -->.*?<!-- END_COMMAND_REFERENCE -->`)
	newContent := re.ReplaceAllString(string(content), 
		fmt.Sprintf("<!-- START_COMMAND_REFERENCE -->\n%s\n<!-- END_COMMAND_REFERENCE -->", formattedHelp))

	// 6. Write back to README.md
	err = os.WriteFile(readmePath, []byte(newContent), 0644)
	if err != nil {
		log.Fatalf("failed to write README: %v", err)
	}

	fmt.Println("Documentation updated successfully!")
}
