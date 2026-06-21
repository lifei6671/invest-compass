import type { TaskEvent } from "../types";

type TaskEventTimelineProps = {
  events: TaskEvent[];
};

export function TaskEventTimeline({ events }: TaskEventTimelineProps) {
  return (
    <div className="task-event-timeline">
      {events.map((event) => (
        <div className={`task-event-item ${event.pending ? "task-event-item-pending" : ""}`} key={event.id}>
          <time>{event.time}</time>
          <span className="task-event-dot" />
          <div className="task-event-content">
            <strong>{event.eventType}</strong>
            <p>{event.description}</p>
          </div>
        </div>
      ))}
    </div>
  );
}
