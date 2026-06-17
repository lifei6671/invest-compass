package task

import (
	"fmt"
	"strings"
)

// FormatSSEEvent 将任务事件编码为标准 SSE 帧。
func FormatSSEEvent(event Event) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("id: %d\n", event.ID))
	builder.WriteString("event: ")
	builder.WriteString(string(event.Type))
	builder.WriteString("\n")
	for _, line := range strings.Split(SanitizeEventPayload(event.Payload), "\n") {
		builder.WriteString("data: ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
	return builder.String()
}

// FormatSSEReplayFrames 按 afterEventID 补拉事件，并编码为 SSE 帧列表。
func FormatSSEReplayFrames(events []Event, afterEventID int64) []string {
	replayed := ReplayEvents(events, afterEventID)
	frames := make([]string, 0, len(replayed))
	for _, event := range replayed {
		frames = append(frames, FormatSSEEvent(event))
	}
	return frames
}
