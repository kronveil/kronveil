import { formatDistanceToNow } from 'date-fns';
import clsx from 'clsx';

interface TimelineEvent {
  id: string;
  timestamp: string;
  type: 'anomaly' | 'incident' | 'remediation' | 'info';
  title: string;
  description: string;
}

const typeStyles = {
  anomaly: 'border-yellow-500 bg-yellow-500/10',
  incident: 'border-red-500 bg-red-500/10',
  remediation: 'border-green-500 bg-green-500/10',
  info: 'border-blue-500 bg-blue-500/10',
};

interface EventTimelineProps {
  events: TimelineEvent[];
}

export default function EventTimeline({ events }: EventTimelineProps) {
  return (
    <div className="space-y-3">
      {events.map((event) => (
        <div key={event.id} className="flex gap-3">
          <div className="flex flex-col items-center">
            <div className={clsx('w-2.5 h-2.5 rounded-full border-2 mt-1.5', typeStyles[event.type])} />
            <div className="w-px flex-1 bg-[rgba(99,179,237,0.08)]" />
          </div>
          <div className="pb-4">
            <div className="flex items-center gap-2 mb-0.5">
              <span className="text-sm font-medium text-gray-200">{event.title}</span>
              <span className="text-[10px] font-mono text-[#718096]">
                {formatDistanceToNow(new Date(event.timestamp), { addSuffix: true })}
              </span>
            </div>
            <p className="text-xs text-[#718096] leading-relaxed">{event.description}</p>
          </div>
        </div>
      ))}
    </div>
  );
}
