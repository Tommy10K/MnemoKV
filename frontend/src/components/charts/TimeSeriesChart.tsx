import { Area, AreaChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts"

type Point = { t: number } & Record<string, number>

type Props = {
  data: Point[]
  dataKey: string
  color?: string
  format?: (value: number) => string
  height?: number
}

export function TimeSeriesChart({
  data,
  dataKey,
  color = "#10b981",
  format,
  height = 180,
}: Props) {
  if (data.length === 0) {
    return (
      <div
        className="flex items-center justify-center rounded-md border border-dashed border-[#1f2937] text-sm text-[#6b7280]"
        style={{ height }}
      >
        waiting for data…
      </div>
    )
  }

  return (
    <div style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <defs>
            <linearGradient id={`grad-${dataKey}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.4} />
              <stop offset="100%" stopColor={color} stopOpacity={0} />
            </linearGradient>
          </defs>
          <XAxis
            dataKey="t"
            tick={{ fill: "#6b7280", fontSize: 11 }}
            tickFormatter={(v: number) => new Date(v * 1000).toLocaleTimeString()}
            minTickGap={40}
          />
          <YAxis
            tick={{ fill: "#6b7280", fontSize: 11 }}
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
    </div>
  )
}
