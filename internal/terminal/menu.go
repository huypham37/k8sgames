package terminal

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/huypham37/k8sgames/internal/game"
)

type Menu struct {
	input  *bufio.Reader
	output io.Writer
}

func NewMenu(input io.Reader, output io.Writer) *Menu {
	return &Menu{input: bufio.NewReader(input), output: output}
}

func (m *Menu) Select(challenges []game.Challenge, completed func(string) bool) ([]string, error) {
	for {
		done := 0
		fmt.Fprintln(m.output, "\nK8s Games — Challenges")
		for index, challenge := range challenges {
			mark := " "
			if completed(challenge.ID) {
				mark = "✓"
				done++
			}
			fmt.Fprintf(m.output, "  %d. [%s] %-20s %s\n", index+1, mark, challenge.Title, challenge.Objective)
		}
		fmt.Fprintf(m.output, "\nProgress: %d/%d\n", done, len(challenges))
		fmt.Fprint(m.output, "Select a number, (a)ll remaining, or (q)uit: ")

		line, err := m.input.ReadString('\n')
		if err != nil {
			return nil, err
		}
		choice := strings.TrimSpace(line)
		switch choice {
		case "q", "quit", "exit":
			return nil, nil
		case "a", "all":
			remaining := make([]string, 0, len(challenges)-done)
			for _, challenge := range challenges {
				if !completed(challenge.ID) {
					remaining = append(remaining, challenge.ID)
				}
			}
			if len(remaining) > 0 {
				return remaining, nil
			}
			fmt.Fprintln(m.output, "All challenges are already complete.")
		default:
			index, err := strconv.Atoi(choice)
			if err == nil && index > 0 && index <= len(challenges) {
				return []string{challenges[index-1].ID}, nil
			}
			fmt.Fprintln(m.output, "Invalid selection.")
		}
	}
}
