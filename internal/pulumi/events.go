package pulumi

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

var (
	opColors = map[apitype.OpType]*color.Color{
		apitype.OpCreate:            color.New(color.FgGreen, color.Bold),
		apitype.OpUpdate:            color.New(color.FgYellow, color.Bold),
		apitype.OpDelete:            color.New(color.FgRed, color.Bold),
		apitype.OpReplace:           color.New(color.FgMagenta, color.Bold),
		apitype.OpCreateReplacement: color.New(color.FgGreen),
		apitype.OpDeleteReplaced:    color.New(color.FgRed),
		apitype.OpSame:             color.New(color.Faint),
		apitype.OpRefresh:          color.New(color.FgCyan),
	}
	opSymbols = map[apitype.OpType]string{
		apitype.OpCreate:            "+",
		apitype.OpUpdate:            "~",
		apitype.OpDelete:            "-",
		apitype.OpReplace:           "±",
		apitype.OpCreateReplacement: "+",
		apitype.OpDeleteReplaced:    "-",
		apitype.OpSame:             " ",
		apitype.OpRefresh:          "↻",
	}
	diagError = color.New(color.FgRed)
	diagWarn  = color.New(color.FgYellow)
	summary   = color.New(color.Bold)
)

func opColor(op apitype.OpType) *color.Color {
	if c, ok := opColors[op]; ok {
		return c
	}
	return color.New(color.Reset)
}

func opSymbol(op apitype.OpType) string {
	if s, ok := opSymbols[op]; ok {
		return s
	}
	return "?"
}

// shortURN extracts the last type::name segment from a URN.
func shortURN(urn string) string {
	parts := strings.Split(urn, "::")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "::" + parts[len(parts)-1]
	}
	return urn
}

// streamEvents processes Pulumi engine events and prints a clean summary.
// Returns a WaitGroup that completes when the channel is drained.
func streamEvents(ch <-chan events.EngineEvent) *sync.WaitGroup {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ev := range ch {
			if ev.ResOutputsEvent != nil {
				meta := ev.ResOutputsEvent.Metadata
				if meta.Op == apitype.OpSame {
					continue
				}
				c := opColor(meta.Op)
				c.Printf("    %s %s %s\n", opSymbol(meta.Op), string(meta.Op), shortURN(meta.URN))
			}

			if ev.DiagnosticEvent != nil {
				d := ev.DiagnosticEvent
				msg := strings.TrimSpace(d.Message)
				if msg == "" {
					continue
				}
				switch d.Severity {
				case "error":
					diagError.Printf("    ✗ %s\n", msg)
				case "warning":
					diagWarn.Printf("    ⚠ %s\n", msg)
				}
			}

			if ev.SummaryEvent != nil {
				s := ev.SummaryEvent
				var parts []string
				order := []apitype.OpType{
					apitype.OpCreate, apitype.OpUpdate, apitype.OpDelete,
					apitype.OpReplace, apitype.OpSame,
				}
				for _, op := range order {
					if count, ok := s.ResourceChanges[op]; ok && count > 0 {
						parts = append(parts, fmt.Sprintf("%d %s", count, op))
					}
				}
				if len(parts) > 0 {
					summary.Printf("    %s\n", strings.Join(parts, ", "))
				}
			}
		}
	}()
	return &wg
}
