import { Area, AreaChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts"

type Point = { t: number } & Record<string, number>

type Props = {
  data: Point[]
  dataKey: string
  color?: string
  format?: (value: number) => string
  height?: number
  ariaLabel: string
}

export function TimeSeriesChart({
  data,
  dataKey,
  color = "#10b981",
  format,
  height = 180,
  ariaLabel,
}: Props) {
  if (data.length === 0) {
    return (
      <div
        className="flex items-center justify-center rounded-md border border-dashed border-[#1f2937] text-sm text-[#8b949e]"
        style={{ height }}
      >
        waiting for data…
      </div>
    )
  }

  return (
    <div className="min-w-0" role="img" aria-label={ariaLabel} style={{ height }}>
      <ResponsiveContainer width="100%" height="100%" minWidth={0}>
        <AreaChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id={`grad-${dataKey}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.4} />
              <stop offset="100%" stopColor={color} stopOpacity={0} />
            </linearGradient>
          </defs>
          <XAxis
            dataKey="t"
            tick={{ fill: "#8b949e", fontSize: 11 }}
            tickFormatter={(v: number) => new Date(v * 1000).toLocaleTimeString()}
            minTickGap={40}
          />
          <YAxis
            tick={{ fill: "#8b949e", fontSize: 11 }}
            tickFormatter={(v: number) => (format ? format(v) : String(v))}
            width={60}
          />
          <Tooltip
            contentStyle={{
              background: "#0b0f17",
              border: "1px solid #1f2937",
              color: "#e6edf3",
              fontSize: 12,
            }}
            labelFormatter={(v) => new Date(Number(v) * 1000).toLocaleTimeString()}
            formatter={(value) => [format ? format(Number(value)) : value, dataKey]}
          />
          <Area
            type="monotone"
            dataKey={dataKey}
            stroke={color}
            fill={`url(#grad-${dataKey})`}
            strokeWidth={2}
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
      <span className="sr-only">{describeSeries(data, dataKey, format)}</span>
    </div>
  )
}

function describeSeries(data: Point[], dataKey: string, format?: (value: number) => string): string {
  const values = data.map((point) => point[dataKey]).filter(Number.isFinite)
  if (values.length === 0) return "No valid data points."
  const show = (value: number) => (format ? format(value) : String(value))
  return `${values.length} points. Latest ${show(values[values.length - 1])}; minimum ${show(Math.min(...values))}; maximum ${show(Math.max(...values))}.`
}
