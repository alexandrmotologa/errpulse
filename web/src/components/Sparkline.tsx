import React from 'react';

interface SparklineProps {
  data?: number[];
  width?: number;
  height?: number;
  color?: string;
}

export const Sparkline: React.FC<SparklineProps> = ({
  data = [],
  width = 120,
  height = 28,
  color = '#10b981',
}) => {
  if (!data || data.length === 0) {
    return <div className="text-xs text-slate-500 italic">No recent activity</div>;
  }

  const max = Math.max(...data, 1);
  const step = width / (data.length - 1 || 1);
  const padding = 2;
  const usableHeight = height - padding * 2;

  const points = data.map((val, idx) => {
    const x = idx * step;
    const y = height - padding - (val / max) * usableHeight;
    return `${x},${y}`;
  });

  const polyline = points.join(' ');
  const areaPoints = `${points[0]} ${polyline} ${width},${height} 0,${height}`;
  const total24h = data.reduce((a, b) => a + b, 0);

  return (
    <div className="flex items-center gap-2" title={`Last 24h events: ${total24h}`}>
      <svg width={width} height={height} className="overflow-visible">
        <defs>
          <linearGradient id={`grad-${color.replace('#', '')}`} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity="0.35" />
            <stop offset="100%" stopColor={color} stopOpacity="0.0" />
          </linearGradient>
        </defs>
        <polygon points={areaPoints} fill={`url(#grad-${color.replace('#', '')})`} />
        <polyline
          fill="none"
          stroke={color}
          strokeWidth="1.75"
          strokeLinecap="round"
          strokeLinejoin="round"
          points={polyline}
        />
        {data[data.length - 1] > 0 && (
          <circle
            cx={width}
            cy={height - padding - (data[data.length - 1] / max) * usableHeight}
            r="2.5"
            fill={color}
          />
        )}
      </svg>
      <span className="text-[11px] font-mono text-slate-400 w-7 text-right">{total24h}</span>
    </div>
  );
};
